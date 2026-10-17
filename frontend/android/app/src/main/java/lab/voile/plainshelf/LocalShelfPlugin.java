package lab.voile.plainshelf;

import android.util.Base64;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.HashMap;
import java.util.Iterator;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import lab.voile.plainshelf.gobind.mobile.Mobile;
import org.json.JSONArray;

/**
 * The local shelf's management calls and its write requests.
 *
 * Shelves are created, renamed and removed only here, never through an HTTP
 * route. A request body is staged chunk by chunk into a cache file
 * (beginBody/appendBody), so neither the bridge nor JNI carries it whole.
 */
@CapacitorPlugin(name = "PlainShelfLocalShelf")
public class LocalShelfPlugin extends Plugin {
    private final Map<String, File> bodies = new ConcurrentHashMap<>();

    @Override
    protected void handleOnDestroy() {
        for (File f : bodies.values()) {
            //noinspection ResultOfMethodCallIgnored
            f.delete();
        }
        bodies.clear();
    }

    @PluginMethod
    public void listShelves(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            JSObject result = new JSObject();
            result.put("shelves", new JSONArray(Mobile.listShelves()));
            call.resolve(result);
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void createShelf(PluginCall call) {
        String name = call.getString("name", "");
        try {
            LocalShelf.ensureStarted(getContext());
            JSObject result = new JSObject();
            result.put("id", Mobile.createShelf(name));
            call.resolve(result);
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void renameShelf(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            Mobile.renameShelf(call.getString("id", ""), call.getString("name", ""));
            call.resolve();
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /** Deletes the shelf's directory with every book in it; the UI confirms first. */
    @PluginMethod
    public void removeShelf(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            Mobile.removeShelf(call.getString("id", ""));
            call.resolve();
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void beginBody(PluginCall call) {
        try {
            File f = LocalShelf.tempFile(getContext(), "req-");
            String id = f.getName();
            bodies.put(id, f);
            JSObject result = new JSObject();
            result.put("bodyId", id);
            call.resolve(result);
        } catch (IOException e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void appendBody(PluginCall call) {
        File f = bodies.get(call.getString("bodyId", ""));
        if (f == null) {
            call.reject("unknown body");
            return;
        }
        byte[] data = Base64.decode(call.getString("data", ""), Base64.DEFAULT);
        try (FileOutputStream out = new FileOutputStream(f, true)) {
            out.write(data);
            call.resolve();
        } catch (IOException e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void discardBody(PluginCall call) {
        File f = bodies.remove(call.getString("bodyId", ""));
        if (f != null) {
            //noinspection ResultOfMethodCallIgnored
            f.delete();
        }
        call.resolve();
    }

    /** One request with a body staged by beginBody, or none. */
    @PluginMethod
    public void request(PluginCall call) {
        String bodyId = call.getString("bodyId");
        File body = bodyId == null ? null : bodies.remove(bodyId);
        if (bodyId != null && body == null) {
            call.reject("unknown body");
            return;
        }
        Map<String, String> headers = new HashMap<>();
        JSObject in = call.getObject("headers", new JSObject());
        Iterator<String> keys = in.keys();
        while (keys.hasNext()) {
            String key = keys.next();
            headers.put(key, in.getString(key));
        }
        try {
            LocalShelf.Result res = LocalShelf.handle(
                getContext(),
                call.getString("method", "GET"),
                call.getString("path", "/"),
                headers,
                body
            );
            JSObject out = new JSObject();
            out.put("status", res.status);
            JSObject outHeaders = new JSObject();
            for (Map.Entry<String, String> h : res.headers.entrySet()) {
                outHeaders.put(h.getKey(), h.getValue());
            }
            out.put("headers", outHeaders);
            // Write responses are small: the large reads are GETs, which the
            // WebView streams through LocalShelfWebViewClient.
            out.put("body", Base64.encodeToString(readAll(res.open()), Base64.NO_WRAP));
            call.resolve(out);
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        } finally {
            if (body != null) {
                //noinspection ResultOfMethodCallIgnored
                body.delete();
            }
        }
    }

    private static byte[] readAll(InputStream in) throws IOException {
        try (InputStream src = in) {
            ByteArrayOutputStream buf = new ByteArrayOutputStream();
            byte[] chunk = new byte[64 * 1024];
            int n;
            while ((n = src.read(chunk)) > 0) {
                buf.write(chunk, 0, n);
            }
            return buf.toByteArray();
        }
    }
}
