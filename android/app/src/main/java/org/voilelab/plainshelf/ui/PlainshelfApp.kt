package org.voilelab.plainshelf.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import org.voilelab.plainshelf.data.BookSummary
import org.voilelab.plainshelf.data.MobileShelfRepository
import org.voilelab.plainshelf.data.SourceSummary

enum class Screen { Library, BookDetail, SourceEditor }

@Composable
fun PlainshelfApp(repository: MobileShelfRepository) {
    var screen by remember { mutableStateOf(Screen.Library) }
    var selectedBookID by remember { mutableStateOf<String?>(null) }
    var selectedSourceID by remember { mutableStateOf<String?>(null) }
    var refreshKey by remember { mutableStateOf(0) }

    DisposableEffect(repository) {
        onDispose { repository.close() }
    }

    MaterialTheme {
        when (screen) {
            Screen.Library -> LibraryScreen(
                repository = repository,
                refreshKey = refreshKey,
                onOpenBook = { bookID ->
                    selectedBookID = bookID
                    screen = Screen.BookDetail
                },
                onChanged = { refreshKey++ },
            )
            Screen.BookDetail -> BookDetailScreen(
                repository = repository,
                bookID = selectedBookID.orEmpty(),
                refreshKey = refreshKey,
                onBack = { screen = Screen.Library },
                onEditSource = { sourceID ->
                    selectedSourceID = sourceID
                    screen = Screen.SourceEditor
                },
                onChanged = { refreshKey++ },
            )
            Screen.SourceEditor -> SourceEditorScreen(
                repository = repository,
                bookID = selectedBookID.orEmpty(),
                sourceID = selectedSourceID.orEmpty(),
                onBack = { screen = Screen.BookDetail },
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun LibraryScreen(
    repository: MobileShelfRepository,
    refreshKey: Int,
    onOpenBook: (String) -> Unit,
    onChanged: () -> Unit,
) {
    val coroutineScope = rememberCoroutineScope()
    var books by remember { mutableStateOf(emptyList<BookSummary>()) }
    var newTitle by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(refreshKey) {
        runCatching { repository.listBooks() }
            .onSuccess { books = it; error = null }
            .onFailure { error = it.message }
    }

    Scaffold(topBar = { TopAppBar(title = { Text("Plainshelf Library") }) }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).padding(16.dp)) {
            error?.let { Text("Error: $it", color = MaterialTheme.colorScheme.error) }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = newTitle,
                    onValueChange = { newTitle = it },
                    label = { Text("New book title") },
                    modifier = Modifier.weight(1f),
                )
                Button(onClick = {
                    if (newTitle.isNotBlank()) {
                        coroutineScope.launch {
                            runCatching { repository.createBook(newTitle.trim()) }
                                .onSuccess { newTitle = ""; onChanged() }
                                .onFailure { error = it.message }
                        }
                    }
                }) { Text("Create") }
            }
            Spacer(Modifier.height(16.dp))
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(books, key = { it.id }) { book ->
                    Card(Modifier.fillMaxWidth().clickable { onOpenBook(book.id) }) {
                        Column(Modifier.padding(16.dp)) {
                            Text(book.title, style = MaterialTheme.typography.titleMedium)
                            Text(book.id, style = MaterialTheme.typography.bodySmall)
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun BookDetailScreen(
    repository: MobileShelfRepository,
    bookID: String,
    refreshKey: Int,
    onBack: () -> Unit,
    onEditSource: (String) -> Unit,
    onChanged: () -> Unit,
) {
    val coroutineScope = rememberCoroutineScope()
    var book by remember(bookID) { mutableStateOf<BookSummary?>(null) }
    var sources by remember(bookID) { mutableStateOf(emptyList<SourceSummary>()) }
    var title by remember(bookID) { mutableStateOf("") }
    var authors by remember(bookID) { mutableStateOf("") }
    var tags by remember(bookID) { mutableStateOf("") }
    var language by remember(bookID) { mutableStateOf("") }
    var comments by remember(bookID) { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(bookID, refreshKey) {
        runCatching {
            val loaded = repository.getBook(bookID)
            val loadedSources = repository.listSources(bookID)
            loaded to loadedSources
        }.onSuccess { (loaded, loadedSources) ->
            book = loaded
            sources = loadedSources
            title = loaded.title
            authors = loaded.authors.joinToString(", ")
            tags = loaded.tags.joinToString(", ")
            language = loaded.language
            comments = loaded.comments
            error = null
        }.onFailure { error = it.message }
    }

    Scaffold(topBar = {
        TopAppBar(
            title = { Text(book?.title ?: "Book") },
            navigationIcon = { TextButton(onClick = onBack) { Text("Back") } },
        )
    }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            error?.let { Text("Error: $it", color = MaterialTheme.colorScheme.error) }
            OutlinedTextField(title, { title = it }, label = { Text("Title") }, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(authors, { authors = it }, label = { Text("Authors, comma separated") }, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(tags, { tags = it }, label = { Text("Tags, comma separated") }, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(language, { language = it }, label = { Text("Language") }, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(comments, { comments = it }, label = { Text("Comments") }, modifier = Modifier.fillMaxWidth())
            Button(onClick = {
                coroutineScope.launch {
                    runCatching {
                        repository.updateBook(
                            bookID = bookID,
                            title = title,
                            authors = authors.csvValues(),
                            tags = tags.csvValues(),
                            language = language,
                            comments = comments,
                        )
                    }.onSuccess { onChanged() }.onFailure { error = it.message }
                }
            }) { Text("Save metadata") }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Sources", style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
                Button(onClick = {
                    coroutineScope.launch {
                        runCatching { repository.createSource(bookID) }
                            .onSuccess { onChanged() }
                            .onFailure { error = it.message }
                    }
                }) { Text("New source") }
            }
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(sources, key = { it.id }) { source ->
                    Card(Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(source.id)
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Button(onClick = { onEditSource(source.id) }) { Text("Edit") }
                                TextButton(onClick = {
                                    coroutineScope.launch {
                                        runCatching { repository.setCurrentSource(bookID, source.id) }
                                            .onSuccess { onChanged() }
                                            .onFailure { error = it.message }
                                    }
                                }) { Text("Set current") }
                            }
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SourceEditorScreen(
    repository: MobileShelfRepository,
    bookID: String,
    sourceID: String,
    onBack: () -> Unit,
) {
    val coroutineScope = rememberCoroutineScope()
    var content by remember(bookID, sourceID) { mutableStateOf("") }
    var status by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(bookID, sourceID) {
        runCatching { repository.getSourceContent(bookID, sourceID) }
            .onSuccess { content = it; status = null }
            .onFailure { status = "Error: ${it.message}" }
    }

    Scaffold(topBar = {
        TopAppBar(
            title = { Text("Source $sourceID") },
            navigationIcon = { TextButton(onClick = onBack) { Text("Back") } },
        )
    }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            status?.let { Text(it) }
            OutlinedTextField(
                value = content,
                onValueChange = { content = it },
                label = { Text("Source text") },
                modifier = Modifier.fillMaxWidth().weight(1f),
            )
            Button(onClick = {
                coroutineScope.launch {
                    runCatching { repository.updateSourceContent(bookID, sourceID, content) }
                        .onSuccess { status = "Saved" }
                        .onFailure { status = "Error: ${it.message}" }
                }
            }) { Text("Save source") }
        }
    }
}

private fun String.csvValues(): List<String> = split(',').map { it.trim() }.filter { it.isNotEmpty() }
