package lab.voile.plainshelf;

import android.net.Uri;
import android.util.Log;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebView;
import com.getcapacitor.Bridge;
import com.getcapacitor.BridgeWebViewClient;
import java.io.ByteArrayInputStream;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

/**
 * Serves GET and HEAD under LocalShelf.PATH_PREFIX from the embedded server:
 * the API client's reads and every {@code <img src>}. The WebView hands no
 * request body to shouldInterceptRequest, so writes go through
 * LocalShelfPlugin instead.
 */
public class LocalShelfWebViewClient extends BridgeWebViewClient {
    private static final String TAG = "LocalShelf";

    private final Bridge bridge;

    public LocalShelfWebViewClient(Bridge bridge) {
        super(bridge);
        this.bridge = bridge;
    }

    @Override
    public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
        Uri url = request.getUrl();
        String path = url.getEncodedPath();
        String method = request.getMethod();
        if (!"localhost".equals(url.getHost()) || path == null || !path.startsWith(LocalShelf.PATH_PREFIX + "/")) {
            return super.shouldInterceptRequest(view, request);
        }
        if (!"GET".equals(method) && !"HEAD".equals(method)) {
            return error(405, "Writes to the local shelf go through the LocalShelf plugin");
        }

        Map<String, String> headers = new HashMap<>();
        for (Map.Entry<String, String> h : request.getRequestHeaders().entrySet()) {
            // WebResourceResponse cannot carry a 3xx, so never ask for a 304.
            String name = h.getKey();
            if (!"If-None-Match".equalsIgnoreCase(name) && !"If-Modified-Since".equalsIgnoreCase(name)) {
                headers.put(name, h.getValue());
            }
        }
        String target = path.substring(LocalShelf.PATH_PREFIX.length());
        String query = url.getEncodedQuery();
        if (query != null) {
            target += "?" + query;
        }

        try {
            LocalShelf.Result res = LocalShelf.handle(bridge.getContext(), method, target, headers, null);
            int status = res.status;
            if (status >= 300 && status < 400) {
                // Not served by the API; refuse instead of crashing the WebView.
                res.open().close();
                return error(502, "Unexpected redirect " + status);
            }
            String contentType = res.headers.get("Content-Type");
            String mime = "application/octet-stream";
            String encoding = null;
            if (contentType != null && !contentType.isEmpty()) {
                String[] parts = contentType.split(";");
                mime = parts[0].trim();
                for (int i = 1; i < parts.length; i++) {
                    String p = parts[i].trim();
                    if (p.toLowerCase().startsWith("charset=")) {
                        encoding = p.substring("charset=".length());
                    }
                }
            }
            return new WebResourceResponse(mime, encoding, status, reason(status), res.headers, res.open());
        } catch (Exception e) {
            Log.e(TAG, "request failed: " + method + " " + target, e);
            return error(500, String.valueOf(e.getMessage()));
        }
    }

    private static WebResourceResponse error(int status, String message) {
        Map<String, String> headers = new HashMap<>();
        headers.put("Content-Type", "text/plain; charset=utf-8");
        return new WebResourceResponse(
            "text/plain",
            "utf-8",
            status,
            reason(status),
            headers,
            new ByteArrayInputStream(message.getBytes(StandardCharsets.UTF_8))
        );
    }

    /** WebResourceResponse refuses an empty reason phrase. */
    private static String reason(int status) {
        return status < 400 ? "OK" : "Error";
    }
}
