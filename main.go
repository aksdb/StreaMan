package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/alecthomas/kong"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi"
	"golang.org/x/net/webdav"
)

type Cli struct {
	Prefix        string `help:"HTTP path prefix."`
	DataDir       string `help:"Data directory." default:"data"`
	ListenAddress string `help:"Listen address." default:":3000"`
	NoEncode      bool   `help:"Disable encoding to x265." default:"false"`

	recorder *recorder
}

func main() {
	var cli Cli
	kong.Parse(&cli)

	davHandler := &webdav.Handler{
		Prefix:     cli.Prefix + "/dav/",
		FileSystem: webdav.Dir(cli.DataDir),
		LockSystem: webdav.NewMemLS(),
	}

	cli.recorder = NewRecorder(cli.DataDir)

	r := chi.NewRouter()

	baseMux := http.NewServeMux()
	baseMux.Handle(davHandler.Prefix, davHandler)
	baseMux.Handle(cli.Prefix+"/", r)

	r.Route(cli.Prefix+"/", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			model, err := buildModel(&cli)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := pageTemplate.Execute(w, model); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
		r.Handle("/files/*", http.StripPrefix(cli.Prefix+"/files/", http.FileServer(http.Dir(cli.DataDir))))

		r.Post("/record", func(w http.ResponseWriter, r *http.Request) {
			channel := r.FormValue("channel")
			if channel == "" {
				http.Error(w, "invalid channel", http.StatusBadRequest)
				return
			}
			transcode := !cli.NoEncode && r.FormValue("transcode") == "on"
			cli.recorder.Record(channel, transcode)
			http.Redirect(w, r, "./", http.StatusFound)
		})

		r.Post("/stop-recording", func(w http.ResponseWriter, r *http.Request) {
			id := r.FormValue("id")
			if err := cli.recorder.Stop(id); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "./", http.StatusFound)
		})

		r.Post("/delete-failure", func(w http.ResponseWriter, r *http.Request) {
			id := r.FormValue("id")
			cli.recorder.RemoveError(id)
			http.Redirect(w, r, "./", http.StatusFound)
		})
	})

	if err := http.ListenAndServe(cli.ListenAddress, baseMux); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

type RecordingModel struct {
	Id       string
	Name     string
	Duration string
}

type FailureModel struct {
	Id     string
	Name   string
	Time   string
	Reason string
}

type FileModel struct {
	Name       string
	URLEncoded string
	Size       string
}

type Model struct {
	Recordings   []RecordingModel
	Failures     []FailureModel
	Files        []FileModel
	CanTranscode bool
}

func buildModel(cli *Cli) (Model, error) {
	var model Model

	files, err := ioutil.ReadDir(cli.DataDir)
	if err != nil {
		return Model{}, fmt.Errorf("cannot list directory: %w", err)
	}

	for _, f := range files {
		if !f.IsDir() {
			model.Files = append(model.Files, FileModel{
				Name:       f.Name(),
				URLEncoded: url.PathEscape(f.Name()),
				Size:       humanize.Bytes(uint64(f.Size())),
			})
		}
	}

	cli.recorder.mtx.Lock()
	defer cli.recorder.mtx.Unlock()

	for id, rec := range cli.recorder.running {
		model.Recordings = append(model.Recordings, RecordingModel{
			Id:       id,
			Name:     rec.filename,
			Duration: time.Now().Sub(rec.startTime).Round(time.Second).String(),
		})
	}

	for id, rec := range cli.recorder.errored {
		model.Failures = append(model.Failures, FailureModel{
			Id:     id,
			Name:   rec.filename,
			Time:   rec.startTime.Format(time.RFC1123),
			Reason: rec.err.Error(),
		})
	}

	model.CanTranscode = !cli.NoEncode

	return model, nil
}

var pageTemplate = template.Must(template.New("page").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>StreaMan</title>
</head>
<body>

<h1>Active Recordings</h1>

<table>
    <thead>
    <tr>
        <td>Name</td>
        <td>Time</td>
        <td>Action</td>
    </tr>
    </thead>
    <tbody>
    {{ range $recording := .Recordings }}
        <tr>
            <td>{{ $recording.Name }}</td>
            <td>{{ $recording.Duration }}</td>
            <td>
                <form style="display: inline;" action="./stop-recording" method="post">
                    <input type="hidden" name="id" value="{{ $recording.Id }}"/>
                    <input type="submit" value="Stop"/>
                </form>
            </td>
        </tr>
    {{ end }}
    </tbody>
</table>

<h1>Record</h1>

<form action="./record" method="post">
    <div>
        <label for="channel">Channel: </label> <input name="channel" id="channel"/>
    </div>
    {{ if .CanTranscode }}
        <div>
            <label for="transcode">Transcode video to h265</label>
            <input type="checkbox" name="transcode" id="transcode"/>
        </div>
    {{ end }}
    <div>
        <input type="submit" value="Record"/>
    </div>
</form>

<h1>Failed Recordings</h1>

<table>
    <thead>
    <tr>
        <td>Name</td>
        <td>Start</td>
        <td>Reason</td>
        <td>Action</td>
    </tr>
    </thead>
    <tbody>
    {{ range $failure := .Failures }}
        <tr>
            <td>{{ $failure.Name }}</td>
            <td>{{ $failure.Time }}</td>
            <td>{{ $failure.Reason }}</td>
            <td>
                <form style="display: inline;" action="./delete-failure" method="post">
                    <input type="hidden" name="id" value="{{ $failure.Id }}"/>
                    <input type="submit" value="Delete"/>
                </form>
            </td>
        </tr>
    {{ end }}
    </tbody>
</table>

<h1>Old Recordings</h1>

{{ range $file := .Files }}
    <a href="./files/{{ $file.URLEncoded }}">{{ $file.Name }}</a> (Size: {{ $file.Size }})<br/>
{{ end }}

</body>
</html>
`))
