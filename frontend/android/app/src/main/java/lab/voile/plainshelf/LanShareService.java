package lab.voile.plainshelf;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.net.ConnectivityManager;
import android.net.LinkAddress;
import android.net.LinkProperties;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.Uri;
import android.net.nsd.NsdManager;
import android.net.nsd.NsdServiceInfo;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.util.Log;
import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;
import androidx.core.app.ServiceCompat;
import java.net.Inet4Address;
import java.net.InetAddress;
import lab.voile.plainshelf.gobind.mobile.Mobile;
import org.json.JSONObject;

/**
 * Keeps the LAN share open while it runs: the foreground notification with the
 * URL and a Stop button, the mDNS record, and the two ways it closes itself (no
 * admitted request for IDLE_LIMIT_MS, or the network it is bound to going away).
 *
 * Go opens and closes the listener (Mobile.startLANShare, called by the
 * plugin); this service ends it whenever the service ends, so no listener
 * outlives the notification. START_NOT_STICKY: a killed process comes back not
 * sharing.
 */
public class LanShareService extends Service {
    static final String ACTION_START = "lab.voile.plainshelf.action.LAN_SHARE_START";
    static final String ACTION_STOP = "lab.voile.plainshelf.action.LAN_SHARE_STOP";
    static final String EXTRA_HOST = "host";

    static final long IDLE_LIMIT_MS = 30 * 60 * 1000L;
    private static final long IDLE_CHECK_MS = 60 * 1000L;

    private static final String TAG = "PlainShelfLanShare";
    private static final String CHANNEL_ID = "lan_share";
    private static final int NOTIFICATION_ID = 2001;
    private static final int ENDED_NOTIFICATION_ID = 2002;
    /** DNS-SD's WebDAV type; the TXT path names where the export is. */
    private static final String NSD_TYPE = "_webdav._tcp";

    /** Told whenever the share starts or ends; set by LanSharePlugin. */
    interface Listener {
        void onChanged();
    }

    private static volatile Listener listener;
    /** Why the last share ended on its own, for the settings page; null when it did not. */
    private static volatile String endedReason;

    static void setListener(Listener l) {
        listener = l;
    }

    static String endedReason() {
        return endedReason;
    }

    private final Handler main = new Handler(Looper.getMainLooper());
    private String host;
    /** The default network host was taken from; another one, even with the same address, ends the share. */
    private Network network;
    private NsdManager.RegistrationListener nsdListener;
    private ConnectivityManager.NetworkCallback networkCallback;
    private boolean ending;

    private final Runnable idleCheck = new Runnable() {
        @Override
        public void run() {
            long idle = Mobile.lanShareIdleMillis();
            if (idle < 0) {
                end(null);
            } else if (idle >= IDLE_LIMIT_MS) {
                end(getString(R.string.lan_share_ended_idle));
            } else {
                main.postDelayed(this, IDLE_CHECK_MS);
            }
        }
    };

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String action = intent == null ? null : intent.getAction();
        if (ACTION_STOP.equals(action)) {
            // Promised by startForegroundService even when stopping straight away.
            startForegroundCompat(buildNotification(""));
            end(null);
            return START_NOT_STICKY;
        }
        String url = runningUrl();
        if (url == null) {
            startForegroundCompat(buildNotification(""));
            end(null);
            return START_NOT_STICKY;
        }
        endedReason = null;
        startForegroundCompat(buildNotification(url));
        String nextHost = intent == null ? null : intent.getStringExtra(EXTRA_HOST);
        if (nsdListener == null || nextHost == null || !nextHost.equals(host)) {
            host = nextHost;
            network = getSystemService(ConnectivityManager.class).getActiveNetwork();
            unregisterNsd();
            registerNsd(url);
        }
        watchNetwork();
        checkNetwork();
        main.removeCallbacks(idleCheck);
        main.postDelayed(idleCheck, IDLE_CHECK_MS);
        notifyChanged();
        return START_NOT_STICKY;
    }

    @Override
    public void onDestroy() {
        // Also reached when the system or the user stops the service.
        teardown();
        super.onDestroy();
    }

    /** Ends the share; reason, when given, is posted as a notification. */
    private void end(String reason) {
        if (ending) {
            return;
        }
        ending = true;
        teardown();
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE);
        if (reason != null) {
            endedReason = reason;
            postEnded(reason);
        }
        stopSelf();
        notifyChanged();
    }

    private void teardown() {
        main.removeCallbacks(idleCheck);
        unregisterNsd();
        if (networkCallback != null) {
            try {
                getSystemService(ConnectivityManager.class).unregisterNetworkCallback(networkCallback);
            } catch (RuntimeException e) {
                Log.w(TAG, "unregisterNetworkCallback", e);
            }
            networkCallback = null;
        }
        try {
            Mobile.stopLANShare();
        } catch (Exception e) {
            Log.w(TAG, "stopLANShare", e);
        }
        notifyChanged();
    }

    /** The share's URL while Go runs it, else null. */
    private static String runningUrl() {
        try {
            JSONObject st = new JSONObject(Mobile.lanShareStatus());
            return st.optBoolean("running") ? st.optString("url") : null;
        } catch (Exception e) {
            return null;
        }
    }

    private static void notifyChanged() {
        Listener l = listener;
        if (l != null) {
            l.onChanged();
        }
    }

    // --- Network ---

    /**
     * This device's IPv4 address on the default network when that is Wi-Fi or
     * Ethernet, else null: a mobile-data address is no LAN.
     */
    static String lanAddress(Context context) {
        ConnectivityManager cm = context.getSystemService(ConnectivityManager.class);
        if (cm == null) {
            return null;
        }
        Network network = cm.getActiveNetwork();
        NetworkCapabilities caps = network == null ? null : cm.getNetworkCapabilities(network);
        if (
            caps == null ||
            !(caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) || caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET))
        ) {
            return null;
        }
        return ipv4Of(cm.getLinkProperties(network));
    }

    private static String ipv4Of(LinkProperties props) {
        if (props == null) {
            return null;
        }
        for (LinkAddress la : props.getLinkAddresses()) {
            InetAddress a = la.getAddress();
            if (a instanceof Inet4Address && !a.isLoopbackAddress() && !a.isLinkLocalAddress()) {
                return a.getHostAddress();
            }
        }
        return null;
    }

    /** Closes the share once its network is replaced or its address is no longer ours. */
    private void watchNetwork() {
        if (networkCallback != null) {
            return;
        }
        ConnectivityManager cm = getSystemService(ConnectivityManager.class);
        networkCallback = new ConnectivityManager.NetworkCallback() {
            @Override
            public void onLost(Network network) {
                main.post(LanShareService.this::checkNetwork);
            }

            @Override
            public void onLinkPropertiesChanged(Network network, LinkProperties props) {
                main.post(LanShareService.this::checkNetwork);
            }

            @Override
            public void onCapabilitiesChanged(Network network, NetworkCapabilities caps) {
                main.post(LanShareService.this::checkNetwork);
            }
        };
        try {
            cm.registerDefaultNetworkCallback(networkCallback);
        } catch (RuntimeException e) {
            Log.w(TAG, "registerDefaultNetworkCallback", e);
            networkCallback = null;
        }
    }

    private void checkNetwork() {
        if (ending || host == null) {
            return;
        }
        Network active = getSystemService(ConnectivityManager.class).getActiveNetwork();
        if (network == null || !network.equals(active) || !host.equals(lanAddress(this))) {
            end(getString(R.string.lan_share_ended_network));
        }
    }

    // --- mDNS ---

    private void registerNsd(String url) {
        int port = Uri.parse(url).getPort();
        if (port <= 0) {
            return;
        }
        NsdServiceInfo info = new NsdServiceInfo();
        info.setServiceName(getString(R.string.app_name));
        info.setServiceType(NSD_TYPE);
        info.setPort(port);
        info.setAttribute("path", "/dav/");
        nsdListener = new NsdManager.RegistrationListener() {
            @Override
            public void onServiceRegistered(NsdServiceInfo info) {}

            @Override
            public void onRegistrationFailed(NsdServiceInfo info, int code) {
                // The URL in the notification still works without it.
                Log.w(TAG, "mDNS registration failed: " + code);
            }

            @Override
            public void onServiceUnregistered(NsdServiceInfo info) {}

            @Override
            public void onUnregistrationFailed(NsdServiceInfo info, int code) {
                Log.w(TAG, "mDNS unregistration failed: " + code);
            }
        };
        try {
            getSystemService(NsdManager.class).registerService(info, NsdManager.PROTOCOL_DNS_SD, nsdListener);
        } catch (RuntimeException e) {
            Log.w(TAG, "registerService", e);
            nsdListener = null;
        }
    }

    private void unregisterNsd() {
        if (nsdListener == null) {
            return;
        }
        try {
            getSystemService(NsdManager.class).unregisterService(nsdListener);
        } catch (RuntimeException e) {
            Log.w(TAG, "unregisterService", e);
        }
        nsdListener = null;
    }

    // --- Notifications ---

    private void startForegroundCompat(Notification n) {
        // specialUse exists from Android 14; earlier versions take no type.
        int type = Build.VERSION.SDK_INT >= 34 ? ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE : 0;
        ServiceCompat.startForeground(this, NOTIFICATION_ID, n, type);
    }

    private void ensureChannel() {
        if (Build.VERSION.SDK_INT < 26) {
            return;
        }
        NotificationChannel channel = new NotificationChannel(
            CHANNEL_ID,
            getString(R.string.lan_share_channel),
            NotificationManager.IMPORTANCE_LOW
        );
        getSystemService(NotificationManager.class).createNotificationChannel(channel);
    }

    private Notification buildNotification(String url) {
        ensureChannel();
        Intent stop = new Intent(this, LanShareService.class).setAction(ACTION_STOP);
        PendingIntent stopIntent = PendingIntent.getService(this, 0, stop, PendingIntent.FLAG_IMMUTABLE);
        return new NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setContentTitle(getString(R.string.lan_share_running))
            .setContentText(url)
            .setContentIntent(openApp())
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .addAction(0, getString(R.string.lan_share_stop), stopIntent)
            .build();
    }

    private void postEnded(String reason) {
        Notification n = new NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setContentTitle(getString(R.string.lan_share_ended))
            .setContentText(reason)
            .setContentIntent(openApp())
            .setAutoCancel(true)
            .build();
        try {
            NotificationManagerCompat.from(this).notify(ENDED_NOTIFICATION_ID, n);
        } catch (SecurityException e) {
            // POST_NOTIFICATIONS was refused; the settings page still says why.
            Log.i(TAG, "ended notification not posted", e);
        }
    }

    private PendingIntent openApp() {
        Intent open = new Intent(this, MainActivity.class).setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        return PendingIntent.getActivity(this, 0, open, PendingIntent.FLAG_IMMUTABLE);
    }
}
