package lab.voile.plainshelf;

import android.content.Context;
import android.content.SharedPreferences;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Base64;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.KeyStore;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import org.json.JSONObject;

@CapacitorPlugin(name = "PlainShelfSecureStorage")
public class SecureStoragePlugin extends Plugin {
    private static final String PREFERENCES_NAME = "plainshelf_secure_storage";
    private static final String KEY_ALIAS = "lab.voile.plainshelf.secure_storage";
    private static final String ANDROID_KEYSTORE = "AndroidKeyStore";
    private static final String CIPHER_TRANSFORMATION = "AES/GCM/NoPadding";
    private static final int GCM_TAG_LENGTH_BITS = 128;
    private static final String VALUE_SEPARATOR = ":";

    private SharedPreferences preferences;

    @Override
    public void load() {
        preferences = getContext().getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE);
    }

    @PluginMethod
    public void get(PluginCall call) {
        String key = requiredString(call, "key");
        if (key == null) {
            return;
        }

        String encrypted = preferences.getString(key, null);
        JSObject result = new JSObject();
        if (encrypted == null) {
            result.put("value", JSONObject.NULL);
            call.resolve(result);
            return;
        }

        try {
            result.put("value", decrypt(encrypted));
            call.resolve(result);
        } catch (GeneralSecurityException | IllegalArgumentException error) {
            call.reject("Secure storage value could not be decrypted", error);
        }
    }

    @PluginMethod
    public void set(PluginCall call) {
        String key = requiredString(call, "key");
        if (key == null) {
            return;
        }
        String value = requiredString(call, "value");
        if (value == null) {
            return;
        }

        try {
            if (!preferences.edit().putString(key, encrypt(value)).commit()) {
                call.reject("Secure storage value could not be persisted");
                return;
            }
            call.resolve();
        } catch (GeneralSecurityException error) {
            call.reject("Secure storage value could not be encrypted", error);
        }
    }

    @PluginMethod
    public void remove(PluginCall call) {
        String key = requiredString(call, "key");
        if (key == null) {
            return;
        }

        if (!preferences.edit().remove(key).commit()) {
            call.reject("Secure storage value could not be removed");
            return;
        }
        call.resolve();
    }

    private String requiredString(PluginCall call, String name) {
        String value = call.getString(name);
        if (value == null || value.isEmpty()) {
            call.reject(name + " must not be empty");
            return null;
        }
        return value;
    }

    private String encrypt(String plaintext) throws GeneralSecurityException {
        Cipher cipher = Cipher.getInstance(CIPHER_TRANSFORMATION);
        cipher.init(Cipher.ENCRYPT_MODE, getOrCreateKey());
        byte[] ciphertext = cipher.doFinal(plaintext.getBytes(StandardCharsets.UTF_8));
        return Base64.encodeToString(cipher.getIV(), Base64.NO_WRAP)
            + VALUE_SEPARATOR
            + Base64.encodeToString(ciphertext, Base64.NO_WRAP);
    }

    private String decrypt(String storedValue) throws GeneralSecurityException {
        String[] parts = storedValue.split(VALUE_SEPARATOR, 2);
        if (parts.length != 2) {
            throw new GeneralSecurityException("Invalid encrypted value");
        }

        byte[] iv = Base64.decode(parts[0], Base64.NO_WRAP);
        byte[] ciphertext = Base64.decode(parts[1], Base64.NO_WRAP);
        Cipher cipher = Cipher.getInstance(CIPHER_TRANSFORMATION);
        cipher.init(Cipher.DECRYPT_MODE, getOrCreateKey(), new GCMParameterSpec(GCM_TAG_LENGTH_BITS, iv));
        return new String(cipher.doFinal(ciphertext), StandardCharsets.UTF_8);
    }

    private SecretKey getOrCreateKey() throws GeneralSecurityException {
        KeyStore keyStore = KeyStore.getInstance(ANDROID_KEYSTORE);
        try {
            keyStore.load(null);
        } catch (IOException error) {
            throw new GeneralSecurityException("Android Keystore could not be loaded", error);
        }
        KeyStore.Entry existing = keyStore.getEntry(KEY_ALIAS, null);
        if (existing instanceof KeyStore.SecretKeyEntry) {
            return ((KeyStore.SecretKeyEntry) existing).getSecretKey();
        }

        KeyGenerator generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE);
        generator.init(
            new KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setRandomizedEncryptionRequired(true)
                .build()
        );
        return generator.generateKey();
    }
}
