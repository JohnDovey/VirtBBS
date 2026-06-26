// VirtAnd — core/build.gradle.kts
// Pure Kotlin/JVM module: no Android dependency, so it builds and tests with
// a plain JVM toolchain — this is the part of VirtAnd that was actually
// verified to compile and run its tests in the development environment
// (no Android SDK was available there). See ../README.md.
plugins {
    `java-library` // needed for the api(...) dependency configuration below
    kotlin("jvm")
    kotlin("plugin.serialization")
}

kotlin {
    jvmToolchain(17)
}

dependencies {
    implementation(kotlin("stdlib"))
    // api, not implementation: the :app module consumes JsonObject/JsonElement
    // etc. directly (building params for UserApiClient.call), so it needs
    // these types on its own compile classpath via the :core dependency.
    api("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.3")
    testImplementation(kotlin("test"))
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
}
