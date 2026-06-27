package io.virtbbs.virtand.data

data class SessionInfo(
    val userName: String = "",
    val bbsName: String = "",
    val securityLevel: Int = 0,
    val sysop: Boolean = false,
)
