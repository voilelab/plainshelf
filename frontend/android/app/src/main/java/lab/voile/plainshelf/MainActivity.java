package lab.voile.plainshelf;

import android.os.Bundle;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SecureStoragePlugin.class);
        registerPlugin(LocalShelfPlugin.class);
        registerPlugin(LanSharePlugin.class);
        registerPlugin(QRScannerPlugin.class);
        super.onCreate(savedInstanceState);
        // The local shelf's reads are served in-process, not over a socket.
        // No bridge when the device has no WebView.
        if (bridge != null) {
            bridge.setWebViewClient(new LocalShelfWebViewClient(bridge));
        }
    }
}
