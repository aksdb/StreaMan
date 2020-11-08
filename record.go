package main

import (
	"fmt"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"time"

	uuid "github.com/satori/go.uuid"
)

type recording struct {
	filename  string
	cmd       *exec.Cmd
	startTime time.Time
	err       error
}

type recorder struct {
	mtx     sync.Mutex
	dataDir string
	encode  bool
	running map[string]*recording
	errored map[string]*recording
}

func NewRecorder(dataDir string, encode bool) *recorder {
	return &recorder{
		dataDir: dataDir,
		encode:  encode,
		running: map[string]*recording{},
		errored: map[string]*recording{},
	}
}

func (r *recorder) Record(channel string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	rec := &recording{
		filename:  fmt.Sprintf("%s_%s.ts", channel, time.Now().Format("20060102_150405")),
		startTime: time.Now(),
	}
	fullPath := path.Join(r.dataDir, rec.filename)
	encodeParams := "-vcodec copy"
	if r.encode {
		encodeParams = "-vcodec libx265 -crf 28"
	}

	recordCmdline := fmt.Sprintf("streamlink https://twitch.tv/%s best -O | ffmpeg -i pipe:0 -ss 00:00:20.0 %s -acodec copy \"%s\"", channel, encodeParams, fullPath)
	rec.cmd = exec.Command("/bin/bash", "-c", recordCmdline)
	rec.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	id := uuid.NewV4().String()

	r.running[id] = rec

	go func(id string, rec *recording) {
		err := rec.cmd.Start()
		if err == nil {
			err = rec.cmd.Wait()
		}
		r.mtx.Lock()
		defer r.mtx.Unlock()

		delete(r.running, id)
		rec.cmd = nil

		if err != nil {
			rec.err = err
			r.errored[id] = rec
		}
	}(id, rec)
}

func (r *recorder) Stop(id string) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	rec := r.running[id]
	if rec == nil {
		return fmt.Errorf("recording not found")
	}
	if err := syscall.Kill(-rec.cmd.Process.Pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("cannot abort recording")
	}
	return nil
}

func (r *recorder) RemoveError(id string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	delete(r.errored, id)
}
