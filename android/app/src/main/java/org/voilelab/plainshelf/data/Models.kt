package org.voilelab.plainshelf.data

data class BookSummary(
    val id: String,
    val title: String,
    val authors: List<String> = emptyList(),
    val tags: List<String> = emptyList(),
    val language: String = "",
    val comments: String = "",
    val currentSource: String = "",
    val layers: List<String> = emptyList(),
)

data class SourceSummary(
    val id: String,
    val createdAt: String = "",
    val md5Hash: String = "",
)
