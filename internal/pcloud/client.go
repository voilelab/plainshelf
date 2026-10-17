// Package pcloud reads a shelf held in a pCloud account, as an fsutil.ReadFS.
//
// It is the Go twin of frontend/src/api/pcloud/client.ts and follows it on the
// details that matter: the token travels as a Bearer header and never in a
// URL, a recursive listing that pCloud refuses is expanded by hand, and only
// transient failures are retried. Nothing here writes to the account.
package pcloud

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

// The two regions. An account lives in exactly one and the other host will not
// serve it, so the host is part of a session's identity, learned at
// authorization rather than guessed.
const (
	HostUS = "api.pcloud.com"
	HostEU = "eapi.pcloud.com"
)

// HostForLocation maps the locationid that pCloud's token reply carries to the
// API host of that region.
func HostForLocation(locationID int) (string, error) {
	switch locationID {
	case 1:
		return HostUS, nil
	case 2:
		return HostEU, nil
	}
	return "", util.Errorf("unknown pCloud location %d", locationID)
}

// Same budgets as client.ts: metadata fails fast, a transfer may be slow.
const (
	requestTimeout  = 30 * time.Second
	downloadTimeout = 5 * time.Minute
	maxAttempts     = 3
	retryBaseDelay  = 500 * time.Millisecond

	// maxReplyBytes bounds an API reply; a recursive listing of a large shelf
	// is the biggest one.
	maxReplyBytes = 256 << 20
)

// rootFolderID is the account's top-level folder.
const rootFolderID int64 = 0

// Client is a read-only pCloud API client for one account.
type Client struct {
	host  string
	token string
	http  *http.Client

	// Variables so tests run without real backoff.
	retryDelay time.Duration
}

// NewClient returns a client for host, one of HostUS and HostEU (a scheme and
// trailing slash are tolerated). Any other host is refused, since it would be
// handed the token. A nil httpClient uses a default one; the time budgets are
// applied per request either way.
func NewClient(host, token string, httpClient *http.Client) (*Client, error) {
	host = strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(host), "https://"), "http://"), "/")
	token = strings.TrimSpace(token)
	if host != HostUS && host != HostEU {
		return nil, util.Errorf("%q is not a pCloud API host", host)
	}
	if token == "" {
		return nil, util.NewError("pCloud access token is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{host: host, token: token, http: httpClient, retryDelay: retryBaseDelay}, nil
}

// item is a file or folder as listfolder returns it.
type item struct {
	Name     string `json:"name"`
	IsFolder bool   `json:"isfolder"`
	FolderID *int64 `json:"folderid"`
	FileID   *int64 `json:"fileid"`
	Size     int64  `json:"size"`
	// Modified is RFC 1123Z.
	Modified string `json:"modified"`
	Contents []item `json:"contents"`
}

type envelope struct {
	Result *int   `json:"result"`
	Error  string `json:"error"`
}

type listFolderReply struct {
	Metadata *item `json:"metadata"`
}

type linkReply struct {
	Hosts []string `json:"hosts"`
	Path  string   `json:"path"`
}

// call issues one API method and decodes its reply into out, retrying only
// failures classified as transient.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) error {
	var err error
	for attempt := 1; ; attempt++ {
		err = c.callOnce(ctx, method, params, out)
		if err == nil || attempt == maxAttempts || !isRetryable(err) {
			return err
		}
		if err := sleep(ctx, c.retryDelay<<(attempt-1)); err != nil {
			return newTransportError(method, err)
		}
	}
}

func (c *Client) callOnce(ctx context.Context, method string, params url.Values, out any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u := url.URL{Scheme: "https", Host: c.host, Path: "/" + method, RawQuery: params.Encode()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return util.Errorf("%w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	res, err := c.http.Do(req)
	if err != nil {
		return newTransportError(method, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return &Error{Method: method, Status: res.StatusCode, Message: res.Status}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxReplyBytes))
	if err != nil {
		return newTransportError(method, err)
	}

	// Every pCloud reply carries result. A reply without one came from
	// somewhere else — a captive portal, a proxy — and must not read as an
	// empty listing.
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Result == nil {
		return &Error{Method: method, Message: "reply did not come from the pCloud API: " + excerpt(body)}
	}
	if *env.Result != 0 {
		msg := env.Error
		if msg == "" {
			msg = "unknown error"
		}
		return &Error{Method: method, Result: *env.Result, Message: msg}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &Error{Method: method, Message: "unreadable reply: " + err.Error()}
	}
	return nil
}

// listFolder lists one folder by id; recursive fills every nested contents.
func (c *Client) listFolder(ctx context.Context, folderID int64, recursive bool) (*item, error) {
	params := url.Values{"folderid": {strconv.FormatInt(folderID, 10)}}
	if recursive {
		params.Set("recursive", "1")
	}
	var reply listFolderReply
	if err := c.call(ctx, "listfolder", params, &reply); err != nil {
		return nil, err
	}
	if reply.Metadata == nil || !reply.Metadata.IsFolder {
		return nil, &Error{Method: "listfolder", Message: "success reported for folder " + strconv.FormatInt(folderID, 10) + " but no folder returned"}
	}
	return reply.Metadata, nil
}

// listFolderRecursive lists a folder and everything beneath it, in one request
// unless pCloud refuses recursion, in which case the tree is walked by hand.
func (c *Client) listFolderRecursive(ctx context.Context, folderID int64) (*item, error) {
	tree, err := c.listFolder(ctx, folderID, true)
	if err == nil {
		return tree, nil
	}
	var perr *Error
	if !errors.As(err, &perr) || perr.Result != resultRecursiveListingUnsupported {
		return nil, err
	}

	tree, err = c.listFolder(ctx, folderID, false)
	if err != nil {
		return nil, err
	}
	for i := range tree.Contents {
		child := &tree.Contents[i]
		if !child.IsFolder || child.FolderID == nil {
			continue
		}
		sub, err := c.listFolderRecursive(ctx, *child.FolderID)
		if err != nil {
			return nil, err
		}
		child.Contents = orEmpty(sub.Contents)
	}
	return tree, nil
}

// resolveFolderID walks a slash-separated path from the account root one
// segment at a time: listfolder is addressed by id, and a path-based listing
// answers "directory does not exist" for folders that plainly exist.
func (c *Client) resolveFolderID(ctx context.Context, folderPath string) (int64, error) {
	id := rootFolderID
	walked := ""
	for segment := range strings.SplitSeq(folderPath, "/") {
		if segment == "" {
			continue
		}
		listing, err := c.listFolder(ctx, id, false)
		if err != nil {
			return 0, err
		}
		var folders []string
		found := false
		for _, child := range listing.Contents {
			if !child.IsFolder || child.FolderID == nil {
				continue
			}
			if child.Name == segment {
				id, found = *child.FolderID, true
				break
			}
			folders = append(folders, child.Name)
		}
		if !found {
			where := "your account root"
			if walked != "" {
				where = strconv.Quote(walked)
			}
			// Naming what is there turns "we disagree about your account" into
			// a typo or the wrong account, which the user can act on.
			return 0, &Error{
				Method: "listfolder",
				Result: resultDirectoryNotFound,
				Message: "pCloud has no folder named " + strconv.Quote(segment) + " in " + where + ". " +
					describeFolders(folders),
			}
		}
		walked += "/" + segment
	}
	return id, nil
}

// fileLink resolves a file's download URL. Links expire, so one is fetched per
// download and never cached.
func (c *Client) fileLink(ctx context.Context, fileID int64) (string, error) {
	return c.link(ctx, "getfilelink", url.Values{"fileid": {strconv.FormatInt(fileID, 10)}})
}

// zipLink resolves a link to a pCloud-built zip of fileIDs, packed under their
// flat names. getziplink, not savezip: it leaves nothing in the account.
func (c *Client) zipLink(ctx context.Context, fileIDs []int64) (string, error) {
	ids := make([]string, len(fileIDs))
	for i, id := range fileIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	return c.link(ctx, "getziplink", url.Values{"fileids": {strings.Join(ids, ",")}})
}

// link is not retried on its own: download retries the link and the transfer
// as one unit.
func (c *Client) link(ctx context.Context, method string, params url.Values) (string, error) {
	var reply linkReply
	if err := c.callOnce(ctx, method, params, &reply); err != nil {
		return "", err
	}
	if len(reply.Hosts) == 0 || reply.Hosts[0] == "" || reply.Path == "" {
		return "", &Error{Method: method, Message: "no usable download host returned"}
	}
	return (&url.URL{Scheme: "https", Host: reply.Hosts[0], Path: reply.Path}).String(), nil
}

// download fetches a link resolved by resolve, retrying transient failures up
// to the point the body starts. The returned body stays bounded by the
// download budget until it is closed.
func (c *Client) download(label string, resolve func(ctx context.Context) (string, error)) (io.ReadCloser, error) {
	for attempt := 1; ; attempt++ {
		body, err := c.downloadOnce(label, resolve)
		if err == nil || attempt == maxAttempts || !isRetryable(err) {
			return body, err
		}
		if err := sleep(context.Background(), c.retryDelay<<(attempt-1)); err != nil {
			return nil, newTransportError(label, err)
		}
	}
}

func (c *Client) downloadOnce(label string, resolve func(ctx context.Context) (string, error)) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	link, err := resolve(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		cancel()
		return nil, util.Errorf("%w", err)
	}
	// No Authorization: the content host honours the link's own credentials
	// and the API token has no business there.
	res, err := c.http.Do(req)
	if err != nil {
		cancel()
		return nil, newTransportError(label, err)
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		cancel()
		return nil, &Error{Method: label, Status: res.StatusCode, Message: res.Status}
	}
	return &body{ReadCloser: res.Body, label: label, cancel: cancel}, nil
}

// body reports a transfer cut short as the source being unavailable, and ends
// the download budget on Close.
type body struct {
	io.ReadCloser
	label  string
	cancel context.CancelFunc
}

func (b *body) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = newTransportError(b.label, err)
	}
	return n, err
}

func (b *body) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func orEmpty(items []item) []item {
	if items == nil {
		return []item{}
	}
	return items
}

const maxExcerpt = 200

func excerpt(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxExcerpt {
		return strings.ToValidUTF8(s[:maxExcerpt], "") + "…"
	}
	return s
}

const maxListedFolders = 10

func describeFolders(names []string) string {
	if len(names) == 0 {
		return "That level contains no folders at all."
	}
	names = slices.Sorted(slices.Values(names))
	shown := names[:min(len(names), maxListedFolders)]
	quoted := make([]string, len(shown))
	for i, name := range shown {
		quoted[i] = strconv.Quote(name)
	}
	if rest := len(names) - len(shown); rest > 0 {
		return "It contains " + strings.Join(quoted, ", ") + " and " + strconv.Itoa(rest) + " more."
	}
	return "It contains " + strings.Join(quoted, ", ") + "."
}
