// VirtAnd — settings.gradle.kts
//
// Two modules:
//   core — pure Kotlin/JVM business logic (API client, QWK packet parsing,
//          sync engine), no Android dependency, fully buildable/testable
//          with a plain JVM toolchain.
//   app  — the actual Android application (Activities, Room, WorkManager,
//          UI). Requires the Android SDK to even configure; see
//          android/VirtAnd/README.md for build/verification status.
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "VirtAnd"
include(":core")
include(":app")
