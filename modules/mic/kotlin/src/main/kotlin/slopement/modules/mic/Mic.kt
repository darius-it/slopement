package slopement.modules.mic

import android.Manifest
import android.content.pm.PackageManager
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import dev.mygo.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.isActive
import kotlin.coroutines.coroutineContext

// The microphone on Android: AudioRecord at 16 kHz, mono, 16-bit PCM, read in
// chunks of about 100 ms while the flow is collected. Cancelling the flow (Go's
// context ends) stops the recording and releases the microphone.
//
// The RECORD_AUDIO permission is in the manifest (the app's configuration lists
// it) and is asked for at run time, here, the first time. Without it the flow
// ends with an error that says so. Not run on a device yet.

fun makeRecorder(): Recorder = AndroidRecorder()

private class AndroidRecorder : Recorder {
    override fun samples(): Flow<ByteArray> = flow {
        val context = Native.context
        fun granted() = context.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED
        if (!granted()) {
            // Ask with the system's question, then wait for the answer: the
            // permission is seen granted as soon as the user says yes (no result
            // callback is needed to find that out). A no, or no answer within a
            // minute, ends the recording with the reason.
            Native.activity?.requestPermissions(arrayOf(Manifest.permission.RECORD_AUDIO), 4711)
            var waited = 0
            while (!granted() && waited < 60_000 && coroutineContext.isActive) {
                delay(200)
                waited += 200
            }
            if (!granted()) {
                throw IllegalStateException("microphone access was not allowed (turn it on in the system's settings)")
            }
        }
        val rate = 16000
        val chunkBytes = rate / 10 * 2 // 100 ms of 16-bit mono
        val min = AudioRecord.getMinBufferSize(rate, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT)
        if (min <= 0) throw IllegalStateException("no microphone was found")
        val record = AudioRecord(
            MediaRecorder.AudioSource.MIC, rate, AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT, maxOf(min, chunkBytes * 4),
        )
        if (record.state != AudioRecord.STATE_INITIALIZED) {
            record.release()
            throw IllegalStateException("could not start the microphone")
        }
        try {
            record.startRecording()
            val buffer = ByteArray(chunkBytes)
            while (coroutineContext.isActive) {
                var read = 0
                while (read < chunkBytes && coroutineContext.isActive) {
                    val n = record.read(buffer, read, chunkBytes - read)
                    if (n < 0) throw IllegalStateException("the microphone stopped (code $n)")
                    read += n
                }
                if (read == chunkBytes) emit(buffer.copyOf())
            }
        } finally {
            record.stop()
            record.release()
        }
    }.flowOn(Dispatchers.IO)
}
