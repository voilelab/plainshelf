package org.voilelab.plainshelf

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import org.voilelab.plainshelf.data.MobileShelfRepository
import org.voilelab.plainshelf.ui.PlainshelfApp

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val shelfRoot = filesDir.resolve("shelf").absolutePath
        val repository = MobileShelfRepository(shelfRoot)
        setContent {
            PlainshelfApp(repository = repository)
        }
    }
}
