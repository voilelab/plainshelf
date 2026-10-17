package lab.voile.plainshelf;

import android.app.Activity;
import android.content.ContentResolver;
import android.content.Intent;
import android.database.Cursor;
import android.net.Uri;
import android.provider.DocumentsContract;
import android.provider.OpenableColumns;
import android.util.Base64;
import androidx.activity.result.ActivityResult;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.ActivityCallback;
import com.getcapacitor.annotation.CapacitorPlugin;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.HashMap;
import java.util.Iterator;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import lab.voile.plainshelf.gobind.mobile.Mobile;
import org.json.JSONArray;
import org.json.JSONObject;

/**
 * The local shelf's management calls and its write requests.
 *
 * Shelves are created, renamed and removed only here, never through an HTTP
 * route. A request body is staged chunk by chunk into a cache file
 * (beginBody/appendBody), so neither the bridge nor JNI carries it whole.
 * Shelf zips move between a cache file and a Storage Access Framework
 * document the same way: the bridge carries only the outcome.
 */
@CapacitorPlugin(name = "PlainShelfLocalShelf")
public class LocalShelfPlugin extends Plugin {
    private static final String ZIP_TYPE = "application/zip";

    private final Map<String, File> bodies = new ConcurrentHashMap<>();
    /** The zip exportShelf wrote, waiting for the user to pick its destination. */
    private volatile File pendingExport;

    @Override
    protected void handleOnDestroy() {
        for (File f : bodies.values()) {
            //noinspection ResultOfMethodCallIgnored
            f.delete();
        }
        bodies.clear();
        discardPendingExport();
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

    /**
     * Opens a pCloud shelf entry in the embedded server. The token is read from
     * secure storage here, so it never crosses the bridge.
     */
    @PluginMethod
    public void openPCloudShelf(PluginCall call) {
        try {
            String token = SecureStoragePlugin.readShelfToken(getContext(), call.getString("entryId", ""));
            if (token == null || token.trim().isEmpty()) {
                call.reject("no pCloud token is stored for this shelf");
                return;
            }
            LocalShelf.ensureStarted(getContext());
            Mobile.openPCloudShelf(
                call.getString("id", ""),
                call.getString("name", ""),
                call.getString("host", ""),
                token,
                call.getString("folder", ""),
                call.getString("scope", ""),
                Boolean.TRUE.equals(call.getBoolean("showNsfw", false))
            );
            call.resolve();
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void setShelfShowNsfw(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            Mobile.setShelfShowNSFW(call.getString("id", ""), Boolean.TRUE.equals(call.getBoolean("show", false)));
            call.resolve();
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /** Every book's adult-content marks on an open pCloud shelf, whatever the device shows. */
    @PluginMethod
    public void nsfwMarks(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            JSObject result = new JSObject();
            result.put("marks", new JSONObject(Mobile.shelfNSFWMarks(call.getString("id", ""))));
            call.resolve(result);
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /**
     * Whether a pCloud folder is a shelf, and its book count. Runs during setup,
     * before an entry stores the token, so the form passes the token it holds.
     */
    @PluginMethod
    public void checkPCloudShelf(PluginCall call) {
        try {
            JSONObject check = new JSONObject(
                Mobile.checkPCloudShelf(
                    call.getString("host", ""),
                    call.getString("token", ""),
                    call.getString("folder", "")
                )
            );
            JSObject result = new JSObject();
            result.put("isShelf", check.getBoolean("is_shelf"));
            result.put("books", check.getInt("books"));
            call.resolve(result);
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /** One level of the shelf-root picker; see checkPCloudShelf for the token. */
    @PluginMethod
    public void listPCloudFolder(PluginCall call) {
        try {
            Long folderId = call.getLong("folderId", 0L);
            JSONObject listing = new JSONObject(
                Mobile.listPCloudFolder(
                    call.getString("host", ""),
                    call.getString("token", ""),
                    folderId == null ? 0L : folderId
                )
            );
            JSObject result = new JSObject();
            result.put("folders", listing.getJSONArray("folders"));
            result.put("isShelf", listing.getBoolean("is_shelf"));
            call.resolve(result);
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /**
     * Writes the shelf to a cache zip, then asks where to save it. Writing first
     * means a busy shelf fails before any document is created; last_export is
     * recorded only once the copy has landed.
     */
    @PluginMethod
    public void exportShelf(PluginCall call) {
        String id = call.getString("id", "");
        File zip = null;
        try {
            LocalShelf.ensureStarted(getContext());
            zip = LocalShelf.tempFile(getContext(), "export-");
            Mobile.exportShelf(id, zip.getAbsolutePath());
        } catch (Exception e) {
            if (zip != null) {
                //noinspection ResultOfMethodCallIgnored
                zip.delete();
            }
            call.reject(String.valueOf(e.getMessage()), e);
            return;
        }
        discardPendingExport();
        pendingExport = zip;
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT)
            .addCategory(Intent.CATEGORY_OPENABLE)
            .setType(ZIP_TYPE)
            .putExtra(Intent.EXTRA_TITLE, call.getString("fileName", "plainshelf.zip"));
        startActivityForResult(call, intent, "exportPicked");
    }

    @ActivityCallback
    private void exportPicked(PluginCall call, ActivityResult result) {
        File zip = pendingExport;
        pendingExport = null;
        Uri uri = pickedUri(result);
        ContentResolver resolver = getContext().getContentResolver();
        if (call == null || zip == null || uri == null) {
            // Cancelled, or the app was recreated while the picker was open.
            if (zip != null) {
                //noinspection ResultOfMethodCallIgnored
                zip.delete();
            }
            if (uri != null) {
                deleteDocument(resolver, uri);
            }
            if (call != null && uri == null) {
                resolveCancelled(call, true);
            } else if (call != null) {
                call.reject("export was interrupted");
            }
            return;
        }
        // Activity results arrive on the main thread; the copy may be large.
        execute(() -> {
            try {
                try (InputStream in = new FileInputStream(zip); OutputStream out = resolver.openOutputStream(uri, "w")) {
                    if (out == null) {
                        throw new IOException("cannot open " + uri);
                    }
                    copy(in, out);
                } catch (IOException e) {
                    // The picker already created the document; drop the partial copy.
                    deleteDocument(resolver, uri);
                    throw e;
                }
                Mobile.markExported(call.getString("id", ""));
                resolveCancelled(call, false);
            } catch (Exception e) {
                call.reject(String.valueOf(e.getMessage()), e);
            } finally {
                //noinspection ResultOfMethodCallIgnored
                zip.delete();
            }
        });
    }

    /** Asks for a shelf zip and makes a new shelf from it, named after the file. */
    @PluginMethod
    public void importShelf(PluginCall call) {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT)
            .addCategory(Intent.CATEGORY_OPENABLE)
            .setType("*/*")
            // File managers disagree on a zip's type, so offer the usual ones.
            .putExtra(
                Intent.EXTRA_MIME_TYPES,
                new String[] { ZIP_TYPE, "application/x-zip-compressed", "application/octet-stream" }
            );
        startActivityForResult(call, intent, "importPicked");
    }

    @ActivityCallback
    private void importPicked(PluginCall call, ActivityResult result) {
        if (call == null) {
            return;
        }
        Uri uri = pickedUri(result);
        if (uri == null) {
            resolveCancelled(call, true);
            return;
        }
        execute(() -> {
            ContentResolver resolver = getContext().getContentResolver();
            File zip = null;
            try {
                String name = shelfName(resolver, uri, call.getString("fallbackName", "Imported shelf"));
                zip = LocalShelf.tempFile(getContext(), "import-");
                try (InputStream in = resolver.openInputStream(uri); OutputStream out = new FileOutputStream(zip)) {
                    if (in == null) {
                        throw new IOException("cannot open " + uri);
                    }
                    // Stop at the cap rather than fill the cache with a file Go would refuse.
                    if (!copy(in, out, Mobile.maxImportBytes())) {
                        throw Mobile.getErrTooLarge();
                    }
                }
                LocalShelf.ensureStarted(getContext());
                String id = Mobile.importShelf(name, zip.getAbsolutePath());
                JSObject out = new JSObject();
                out.put("cancelled", false);
                out.put("id", id);
                out.put("name", name);
                call.resolve(out);
            } catch (Exception e) {
                call.reject(String.valueOf(e.getMessage()), e);
            } finally {
                if (zip != null) {
                    //noinspection ResultOfMethodCallIgnored
                    zip.delete();
                }
            }
        });
    }

    private void discardPendingExport() {
        if (pendingExport != null) {
            //noinspection ResultOfMethodCallIgnored
            pendingExport.delete();
            pendingExport = null;
        }
    }

    private static Uri pickedUri(ActivityResult result) {
        if (result.getResultCode() != Activity.RESULT_OK || result.getData() == null) {
            return null;
        }
        return result.getData().getData();
    }

    private static void resolveCancelled(PluginCall call, boolean cancelled) {
        JSObject out = new JSObject();
        out.put("cancelled", cancelled);
        call.resolve(out);
    }

    private static void deleteDocument(ContentResolver resolver, Uri uri) {
        try {
            DocumentsContract.deleteDocument(resolver, uri);
        } catch (Exception ignored) {
            // A provider without delete support keeps the empty document.
        }
    }

    /** The picked file's name without ".zip", or fallback. */
    private static String shelfName(ContentResolver resolver, Uri uri, String fallback) {
        String name = null;
        try (Cursor c = resolver.query(uri, new String[] { OpenableColumns.DISPLAY_NAME }, null, null, null)) {
            if (c != null && c.moveToFirst()) {
                name = c.getString(0);
            }
        } catch (Exception ignored) {
            // Some providers answer no columns; the fallback covers them.
        }
        if (name == null) {
            return fallback;
        }
        if (name.toLowerCase(Locale.ROOT).endsWith(".zip")) {
            name = name.substring(0, name.length() - 4);
        }
        name = name.trim();
        return name.isEmpty() ? fallback : name;
    }

    private static void copy(InputStream in, OutputStream out) throws IOException {
        copy(in, out, Long.MAX_VALUE);
    }

    /** Copies at most limit bytes; false when in had more. */
    private static boolean copy(InputStream in, OutputStream out, long limit) throws IOException {
        byte[] chunk = new byte[64 * 1024];
        long total = 0;
        int n;
        while ((n = in.read(chunk)) > 0) {
            total += n;
            if (total > limit) {
                return false;
            }
            out.write(chunk, 0, n);
        }
        return true;
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
