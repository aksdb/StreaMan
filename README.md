# StreaMan

This is a very simple (Twitch) stream recorder and server I wrote
so I can record Twitch streams that might be DMCA'd soon (music,
games with music, etc.).

It uses [streamlink](https://streamlink.github.io) and 
[ffmpeg](https://ffmpeg.org) with libx265 to grab the stream in a
more or less sane file size and can then restream.

It serves the files via HTTP for download and via WebDAV for simple
usage in Kodi.

If you need authentication (for example if it is supposed to run
on the open internet), you have to add BasicAuth via your reverse
proxy.

## Compile

To compile, just grab the source and run `go build .`. Requires Go 1.15.

## Usage

```
Usage: streaman

Flags:
  -h, --help                      Show context-sensitive help.
      --prefix=STRING             HTTP path prefix.
      --data-dir="data"           Data directory.
      --listen-address=":3000"    Listen address.

```

Running it with default settings means you access it via
`http://localhost:3000`. There it will serve a very simple UI that
shows you the current recordings and allows you to record a twitch
channel (just type its name and hit *Record*).

If you want to add the current recorded files to Kodi, simply add
a new WebDAV source and point it to `http://<your-ip>:3000/dav/`.
