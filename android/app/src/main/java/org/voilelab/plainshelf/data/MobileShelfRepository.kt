package org.voilelab.plainshelf.data

import org.json.JSONArray
import org.json.JSONObject

class MobileShelfRepository(private val rootPath: String) {
    private var shelf: Any? = null

    fun open() {
        if (shelf != null) return
        val facade = Class.forName("shelfmobile.Shelfmobile")
        val openMethod = facade.methods.firstOrNull { method ->
            method.name.equals("open", ignoreCase = true) && method.parameterTypes.size == 1
        } ?: error("shelfmobile open method not found")
        shelf = openMethod.invoke(null, rootPath)
    }

    fun close() {
        shelf?.call("close")
        shelf = null
    }

    fun listBooks(): List<BookSummary> {
        open()
        return JSONArray(shelf!!.callString("listBooksJSON")).mapBookObjects { it.toBookSummary() }
    }

    fun createBook(title: String): BookSummary {
        open()
        val request = JSONObject().put("title", title).toString()
        return JSONObject(shelf!!.callString("createBookJSON", request)).toBookSummary()
    }

    fun getBook(bookID: String): BookSummary {
        open()
        return JSONObject(shelf!!.callString("getBookJSON", bookID)).toBookSummary()
    }

    fun updateBook(
        bookID: String,
        title: String,
        authors: List<String>,
        tags: List<String>,
        language: String,
        comments: String,
    ): BookSummary {
        open()
        val request = JSONObject()
            .put("title", title)
            .put("authors", JSONArray(authors))
            .put("tags", JSONArray(tags))
            .put("language", language)
            .put("comments", comments)
        return JSONObject(shelf!!.callString("updateBookJSON", bookID, request.toString())).toBookSummary()
    }

    fun listSources(bookID: String): List<SourceSummary> {
        open()
        return JSONArray(shelf!!.callString("listSourcesJSON", bookID)).mapSourceObjects { it.toSourceSummary() }
    }

    fun createSource(bookID: String): SourceSummary {
        open()
        return JSONObject(shelf!!.callString("createSourceJSON", bookID)).toSourceSummary()
    }

    fun setCurrentSource(bookID: String, sourceID: String) {
        open()
        shelf!!.call("setCurrentSource", bookID, sourceID)
    }

    fun getSourceContent(bookID: String, sourceID: String): String {
        open()
        return shelf!!.callString("getSourceContent", bookID, sourceID)
    }

    fun updateSourceContent(bookID: String, sourceID: String, content: String) {
        open()
        shelf!!.call("updateSourceContent", bookID, sourceID, content)
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
