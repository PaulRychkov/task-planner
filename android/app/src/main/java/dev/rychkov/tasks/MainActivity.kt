package dev.rychkov.tasks

import android.app.Activity
import android.app.AlertDialog
import android.content.Context
import android.os.Bundle
import android.view.Menu
import android.view.MenuItem
import android.webkit.JavascriptInterface
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.Toast
import mobile.Mobile

class MainActivity : Activity() {

    private lateinit var web: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        startServer()
        actionBar?.hide()
        web = WebView(this).apply {
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            settings.textZoom = 100
            webViewClient = WebViewClient()
            webChromeClient = WebChromeClient()
            addJavascriptInterface(JsHost(), "AndroidHost")
            loadUrl(Mobile.baseURL())
        }
        val root = FrameLayout(this).apply { fitsSystemWindows = true }
        root.addView(
            web,
            FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT,
            ),
        )
        setContentView(root)
    }

    inner class JsHost {
        @JavascriptInterface
        fun openSync() {
            runOnUiThread { showSyncDialog() }
        }
    }

    override fun onCreateOptionsMenu(menu: Menu): Boolean {
        menu.add(0, MENU_SYNC, 0, getString(R.string.menu_sync))
        return true
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        if (item.itemId == MENU_SYNC) {
            showSyncDialog()
            return true
        }
        return super.onOptionsItemSelected(item)
    }

    private fun prefs() = getSharedPreferences("sync", Context.MODE_PRIVATE)

    private fun startServer() {
        ServerService.launch(this)
        val err = Mobile.start(
            filesDir.absolutePath,
            prefs().getString("url", "") ?: "",
            prefs().getString("token", "") ?: "",
        )
        if (err.isNotEmpty()) {
            Toast.makeText(this, err, Toast.LENGTH_LONG).show()
        }
    }

    private fun showSyncDialog() {
        val urlInput = EditText(this).apply {
            hint = getString(R.string.sync_url_hint)
            setText(prefs().getString("url", ""))
        }
        val tokenInput = EditText(this).apply {
            hint = getString(R.string.sync_token_hint)
            setText(prefs().getString("token", ""))
        }
        val box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 24, 48, 0)
            addView(urlInput)
            addView(tokenInput)
        }
        AlertDialog.Builder(this)
            .setTitle(getString(R.string.menu_sync))
            .setView(box)
            .setPositiveButton(getString(R.string.sync_save)) { _, _ ->
                prefs().edit()
                    .putString("url", urlInput.text.toString().trim().trimEnd('/'))
                    .putString("token", tokenInput.text.toString().trim())
                    .apply()
                Mobile.stop()
                startServer()
                web.reload()
            }
            .setNegativeButton(getString(R.string.sync_cancel), null)
            .show()
    }

    private companion object {
        const val MENU_SYNC = 1
    }
}
