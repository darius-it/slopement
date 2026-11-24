package dev.mygo

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.OpenableColumns
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.util.UUID

/**
 * slopement as a share target: the system's share sheet lists the app (the
 * activity alias in mygo-plugin.json), and what is shared (words, a picture,
 * several files) comes here as an Intent. The shared files are copied into the
 * app's cache at once (the permission to read them ends with the Intent) and Go
 * is told where they are; Go removes them after sending, or when the user
 * cancels. Not run on a device yet.
 */
class ShareTargetPlugin : MyGoPlugin {
    private var listening = false

    private companion object {
        const val MAX_COPY = 50L * 1024 * 1024 + 1 // one byte over the app's limit, so Go can refuse it
    }

    private fun handle(i: Intent) {
        if (!listening) return
        if (i.action != Intent.ACTION_SEND && i.action != Intent.ACTION_SEND_MULTIPLE) return
        val text = i.getStringExtra(Intent.EXTRA_TEXT)
        val uris = ArrayList<Uri>()
        @Suppress("DEPRECATION")
        if (i.action == Intent.ACTION_SEND) {
            val u = if (Build.VERSION.SDK_INT >= 33) i.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java)
            else i.getParcelableExtra<Uri>(Intent.EXTRA_STREAM)
            if (u != null) uris.add(u)
        } else {
            val l = if (Build.VERSION.SDK_INT >= 33) i.getParcelableArrayListExtra(Intent.EXTRA_STREAM, Uri::class.java)
            else i.getParcelableArrayListExtra<Uri>(Intent.EXTRA_STREAM)
            if (l != null) uris.addAll(l)
        }
        // Once: an Activity made again would deliver the same Intent.
        i.action = Intent.ACTION_MAIN
        i.removeExtra(Intent.EXTRA_STREAM)
        i.removeExtra(Intent.EXTRA_TEXT)
        if (text == null && uris.isEmpty()) return

        Thread {
            val files = JSONArray()
            val dir = File(PluginHost.context.cacheDir, "shares").apply { mkdirs() }
            for (u in uris) {
                try {
                    val name = displayName(u)
                    val out = File(dir, UUID.randomUUID().toString().take(8) + "-" + name.replace('/', '_'))
                    PluginHost.context.contentResolver.openInputStream(u)?.use { input ->
                        out.outputStream().use { o ->
                            val buf = ByteArray(64 * 1024)
                            var total = 0L
                            while (true) {
                                val n = input.read(buf)
                                if (n < 0) break
                                total += n
                                if (total > MAX_COPY) break
                                o.write(buf, 0, n)
                            }
                        }
                    } ?: continue
                    files.put(JSONObject().put("path", out.absolutePath).put("name", name)
                        .put("mime", PluginHost.context.contentResolver.getType(u) ?: ""))
                } catch (_: Exception) {
                    // one file that cannot be read does not stop the others
                }
            }
            val o = JSONObject().put("files", files)
            if (text != null) o.put("text", text)
            PluginHost.emit("sharetarget", "share", o)
        }.start()
    }

    private fun displayName(u: Uri): String {
        PluginHost.context.contentResolver.query(u, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) c.getString(0)?.let { if (it.isNotBlank()) return it }
        }
        return u.lastPathSegment?.substringAfterLast('/') ?: "file"
    }

    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        when (method) {
            "listen" -> {
                // Chained after whatever hears Intents already (the push plugin's).
                if (!listening) {
                    listening = true
                    val before = PluginHost.onIntent
                    PluginHost.onIntent = { before?.invoke(it); handle(it) }
                }
                // What opened the app, if it was a share.
                PluginHost.activity?.intent?.let { handle(it) }
                reply.success(null)
            }
            else -> reply.failure(PluginFailure("NoMethod", method))
        }
    }
}
