# Protocol v0 Examples

このディレクトリは protocol v0 の代表的なメッセージ例を格納します。

- session.hello.example.json
- session.welcome.example.json
- audio.chunk.example.json
- audio.end.example.json
- audio.stream_open.example.json
- audio.stream_abort.example.json
- error.invalid-sequence.example.json
- stt.final.example.json
- tts.chunk.example.json
- tts.end.example.json
- tts.stop.example.json
- avatar.expression.example.json
- motion.play.example.json
- conversation.cancel.example.json

- device.audio.test.play.example.json
- device.mic.test.start.example.json
- device.camera.capture.example.json
- device.state.report.request.example.json
- device.state.report.response.example.json

注意:

- 例の timestamp は RFC3339 UTC を使用しています。
- sequence は送信方向ごとに単調増加する前提です。
