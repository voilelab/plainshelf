package lab.voile.plainshelf;

import android.content.Intent;
import android.content.pm.PackageManager;
import androidx.activity.result.ActivityResult;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.ActivityCallback;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.journeyapps.barcodescanner.ScanIntentResult;
import com.journeyapps.barcodescanner.ScanOptions;

/** Bundled ZXing camera scanner; no Play services or downloaded model. */
@CapacitorPlugin(name = "PlainShelfQRScanner")
public class QRScannerPlugin extends Plugin {
    private boolean scanning;

    @PluginMethod
    public void scan(PluginCall call) {
        if (scanning) {
            call.reject("QR scanner is already open");
            return;
        }
        if (!getContext().getPackageManager().hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY)) {
            call.reject("QR scanning requires a camera");
            return;
        }
        ScanOptions options = new ScanOptions()
            .setDesiredBarcodeFormats(ScanOptions.QR_CODE)
            .setBeepEnabled(false)
            .setBarcodeImageEnabled(false)
            .setOrientationLocked(false);
        scanning = true;
        try {
            startActivityForResult(call, options.createScanIntent(getContext()), "scanned");
        } catch (RuntimeException error) {
            scanning = false;
            call.reject("QR scanner is unavailable");
        }
    }

    @ActivityCallback
    private void scanned(PluginCall call, ActivityResult activityResult) {
        scanning = false;
        if (call == null) {
            return;
        }
        Intent data = activityResult.getData();
        if (data != null && data.getBooleanExtra("MISSING_CAMERA_PERMISSION", false)) {
            call.reject("Camera permission is required for QR scanning");
            return;
        }
        ScanIntentResult scan = ScanIntentResult.parseActivityResult(activityResult.getResultCode(), data);
        JSObject result = new JSObject();
        result.put("cancelled", scan.getContents() == null);
        if (scan.getContents() != null) {
            result.put("text", scan.getContents());
        }
        call.resolve(result);
    }
}
