package lab.voile.plainshelf;

import android.Manifest;
import android.content.Intent;
import android.os.Build;
import androidx.core.content.ContextCompat;
import com.getcapacitor.JSArray;
import com.getcapacitor.JSObject;
import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;
import lab.voile.plainshelf.gobind.mobile.Mobile;
import org.json.JSONArray;
import org.json.JSONObject;

/**
 * Switches the LAN share. Go binds the listener here, so a refused address or
 * a busy port fails the call; LanShareService then holds it open.
 */
@CapacitorPlugin(
    name = "PlainShelfLanShare",
    permissions = { @Permission(alias = "notifications", strings = { Manifest.permission.POST_NOTIFICATIONS }) }
)
public class LanSharePlugin extends Plugin {
    @Override
    public void load() {
        LanShareService.setListener(() -> {
            try {
                notifyListeners("lanShareChanged", status());
            } catch (Exception ignored) {
                // The page asks again when it next shows.
            }
        });
    }

    @Override
    protected void handleOnDestroy() {
        LanShareService.setListener(null);
    }

    @PluginMethod
    public void getStatus(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            call.resolve(status());
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /** Asks for the notification permission first; the share starts whatever the answer. */
    @PluginMethod
    public void start(PluginCall call) {
        if (Build.VERSION.SDK_INT >= 33 && getPermissionState("notifications") != PermissionState.GRANTED) {
            requestPermissionForAlias("notifications", call, "afterNotificationPermission");
            return;
        }
        doStart(call);
    }

    @PermissionCallback
    private void afterNotificationPermission(PluginCall call) {
        doStart(call);
    }

    private void doStart(PluginCall call) {
        try {
            JSONArray ids = call.getArray("shelfIds", new JSArray());
            String host = LanShareService.lanAddress(getContext());
            if (host == null) {
                call.reject("not connected to Wi-Fi");
                return;
            }
            LocalShelf.ensureStarted(getContext());
            Mobile.startLANShare(host, ids.toString());
            Intent intent = new Intent(getContext(), LanShareService.class)
                .setAction(LanShareService.ACTION_START)
                .putExtra(LanShareService.EXTRA_HOST, host);
            try {
                ContextCompat.startForegroundService(getContext(), intent);
            } catch (RuntimeException e) {
                // No service, no share: nothing may listen without the notification.
                Mobile.stopLANShare();
                throw e;
            }
            call.resolve(status());
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    @PluginMethod
    public void stop(PluginCall call) {
        try {
            LocalShelf.ensureStarted(getContext());
            getContext().stopService(new Intent(getContext(), LanShareService.class));
            Mobile.stopLANShare();
            call.resolve(status());
        } catch (Exception e) {
            call.reject(String.valueOf(e.getMessage()), e);
        }
    }

    /** Go's LANShareState plus why the last share ended on its own. */
    private static JSObject status() throws Exception {
        JSObject out = JSObject.fromJSONObject(new JSONObject(Mobile.lanShareStatus()));
        String reason = LanShareService.endedReason();
        out.put("endedReason", reason == null ? "" : reason);
        out.put("idleLimitMinutes", LanShareService.IDLE_LIMIT_MS / 60000);
        return out;
    }
}
