package lab.voile.plainshelf;

import android.content.Context;
import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.Map;
import lab.voile.plainshelf.gobind.mobile.Mobile;
import lab.voile.plainshelf.gobind.mobile.Response;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * The embedded Go server (mobile/), shared by LocalShelfPlugin and
 * LocalShelfWebViewClient. Nothing listens on a socket: every request is a
 * call to Mobile.handle, and bodies cross JNI as file paths.
 */
final class LocalShelf {
    /** Path prefix the WebView and the plugin route to the embedded server. */
    static final String PATH_PREFIX = "/__plainshelf_local";

    private static boolean started;

    private LocalShelf() {}

    /** Starts the server on first use; Start itself returns before the first scan. */
    static synchronized void ensureStarted(Context context) throws Exception {
        if (started) {
            return;
        }
        File dataDir = new File(context.getFilesDir(), "local-shelves");
        if (!dataDir.isDirectory() && !dataDir.mkdirs()) {
            throw new IOException("cannot create " + dataDir);
        }
        Mobile.start(dataDir.getAbsolutePath());
        started = true;
    }

    /** A temp file for one body, in the cache directory. */
    static File tempFile(Context context, String prefix) throws IOException {
        File dir = new File(context.getCacheDir(), "local-shelf");
        if (!dir.isDirectory() && !dir.mkdirs()) {
            throw new IOException("cannot create " + dir);
        }
        return File.createTempFile(prefix, ".tmp", dir);
    }

    static final class Result {
        final int status;
        final Map<String, String> headers;
        /** The response body; deleted once opened, so only the stream holds it. */
        final File body;

        Result(int status, Map<String, String> headers, File body) {
            this.status = status;
            this.headers = headers;
            this.body = body;
        }

        /** Opens the body and unlinks the file; the open stream keeps its data. */
        InputStream open() throws IOException {
            InputStream in = new FileInputStream(body);
            //noinspection ResultOfMethodCallIgnored
            body.delete();
            return in;
        }
    }

    /**
     * Serves one request. path is the API path without PATH_PREFIX; bodyFile may
     * be null and is not deleted here.
     */
    static Result handle(Context context, String method, String path, Map<String, String> headers, File bodyFile)
        throws Exception {
        ensureStarted(context);
        File out = tempFile(context, "res-");
        try {
            Response res = Mobile.handle(
                method,
                path,
                new JSONObject(headers).toString(),
                bodyFile == null ? "" : bodyFile.getAbsolutePath(),
                out.getAbsolutePath()
            );
            return new Result((int) res.getStatus(), parseHeaders(res.getHeader()), out);
        } catch (Exception error) {
            //noinspection ResultOfMethodCallIgnored
            out.delete();
            throw error;
        }
    }

    private static Map<String, String> parseHeaders(String json) throws JSONException {
        Map<String, String> out = new LinkedHashMap<>();
        if (json == null || json.isEmpty()) {
            return out;
        }
        JSONObject obj = new JSONObject(json);
        Iterator<String> keys = obj.keys();
        while (keys.hasNext()) {
            String key = keys.next();
            out.put(key, obj.getString(key));
        }
        return out;
    }
}
