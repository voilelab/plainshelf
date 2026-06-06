package org.voilelab.plainshelf.data

import org.json.JSONArray
import org.json.JSONObject
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

class MobileShelfRepository(private val rootPath: String) {
    private val shelfMutex = Mutex()
    private var shelf: Any? = null

    private fun openInternal() {
        if (shelf != null) return
        val facade = Class.forName("shelfmobile.Shelfmobile")
        val openMethod = facade.methods.firstOrNull { method ->
            method.name.equals("open", ignoreCase = true) && method.parameterTypes.size == 1
        } ?: error("shelfmobile open method not found")
        shelf = openMethod.invoke(null, rootPath)
    }

    private fun closeInternal() {
        shelf?.call("close")
        shelf = null
    }

    suspend fun open() = withContext(Dispatchers.IO) {
        shelfMutex.withLock { openInternal() }
    }

    suspend fun close() = withContext(Dispatchers.IO) {
        shelfMutex.withLock { closeInternal() }
    }

    private suspend fun <T> withShelf(block: (Any) -> T): T = withContext(Dispatchers.IO) {
        shelfMutex.withLock {
            openInternal()
            block(shelf ?: error("shelfmobile open returned null"))
        }
    }

    suspend fun listBooks(): List<BookSummary> = withShelf { currentShelf ->
        JSONArray(currentShelf.callString("listBooksJSON")).mapBookObjects { it.toBookSummary() }
    }

    suspend fun createBook(title: String): BookSummary = withShelf { currentShelf ->
        val request = JSONObject().put("title", title).toString()
        JSONObject(currentShelf.callString("createBookJSON", request)).toBookSummary()
    }

    suspend fun getBook(bookID: String): BookSummary = withShelf { currentShelf ->
        JSONObject(currentShelf.callString("getBookJSON", bookID)).toBookSummary()
    }

    suspend fun updateBook(
        bookID: String,
        title: String,
        authors: List<String>,
        tags: List<String>,
        language: String,
        comments: String,
    ): BookSummary = withShelf { currentShelf ->
        val request = JSONObject()
            .put("title", title)
            .put("authors", JSONArray(authors))
            .put("tags", JSONArray(tags))
            .put("language", language)
            .put("comments", comments)
        JSONObject(currentShelf.callString("updateBookJSON", bookID, request.toString())).toBookSummary()
    }

    suspend fun listSources(bookID: String): List<SourceSummary> = withShelf { currentShelf ->
        JSONArray(currentShelf.callString("listSourcesJSON", bookID)).mapSourceObjects { it.toSourceSummary() }
    }

    suspend fun createSource(bookID: String): SourceSummary = withShelf { currentShelf ->
        JSONObject(currentShelf.callString("createSourceJSON", bookID)).toSourceSummary()
    }

    suspend fun setCurrentSource(bookID: String, sourceID: String) = withShelf { currentShelf ->
        currentShelf.call("setCurrentSource", bookID, sourceID)
    }

    suspend fun getSourceContent(bookID: String, sourceID: String): String = withShelf { currentShelf ->
        currentShelf.callString("getSourceContent", bookID, sourceID)
    }

    suspend fun updateSourceContent(bookID: String, sourceID: String, content: String) = withShelf { currentShelf ->
        currentShelf.call("updateSourceContent", bookID, sourceID, content)
    }
}

private fun Any.call(methodName: String, vararg args: Any?): Any? {
    val method = javaClass.methods.firstOrNull { method ->
        method.name == methodName && method.parameterTypes.size == args.size
    } ?: javaClass.methods.firstOrNull { method ->
        method.name.equals(methodName, ignoreCase = true) && method.parameterTypes.size == args.size
    } ?: error("shelfmobile method not found: $methodName/${args.size}")
    return method.invoke(this, *args)
}

private fun Any.callString(methodName: String, vararg args: Any?): String = call(methodName, *args) as String

private fun JSONArray.mapBookObjects(transform: (JSONObject) -> BookSummary): List<BookSummary> =
    (0 until length()).map { transform(getJSONObject(it)) }

private fun JSONArray.mapSourceObjects(transform: (JSONObject) -> SourceSummary): List<SourceSummary> =
    (0 until length()).map { transform(getJSONObject(it)) }

private fun JSONObject.toBookSummary(): BookSummary = BookSummary(
    id = optString("id"),
    title = optString("title"),
    authors = optStringList("authors"),
    tags = optStringList("tags"),
    language = optString("language"),
    comments = optString("comments"),
    currentSource = optString("current_source"),
    layers = optStringList("layers"),
)

private fun JSONObject.toSourceSummary(): SourceSummary = SourceSummary(
    id = optString("id"),
    createdAt = optString("created_at"),
    md5Hash = optString("md5_hash"),
)

private fun JSONObject.optStringList(name: String): List<String> {
    val array = optJSONArray(name) ?: return emptyList()
    return (0 until array.length()).map { array.optString(it) }
}
