package server

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/abjelosevic88/pocket-homelab-still/internal/content"
)

// lessonView is a lesson with where the person is.
type lessonView struct {
	content.Lesson
	Program  string `json:"program"`
	Stage    string `json:"stage"`
	StageIdx int    `json:"stage_idx"`
	Index    int    `json:"index"`
	Done     bool   `json:"done"`
	DoneAt   string `json:"done_at,omitempty"`
	Locked   bool   `json:"locked"`
	Current  bool   `json:"current"`
}

// stageView is a stage with its lessons and how far the person got.
type stageView struct {
	Key      string       `json:"key"`
	Name     string       `json:"name"`
	Tagline  string       `json:"tagline"`
	Lessons  []lessonView `json:"lessons"`
	Done     int          `json:"done"`
	Total    int          `json:"total"`
	Complete bool         `json:"complete"`
	Locked   bool         `json:"locked"`
	Minutes  float64      `json:"minutes"` // practice time of the whole stage
}

// programView is a program with the person's progress.
type programView struct {
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Tagline     string      `json:"tagline"`
	Description string      `json:"description"`
	Color       string      `json:"color"`
	Icon        string      `json:"icon"`
	Done        int         `json:"done"`
	Total       int         `json:"total"`
	Percent     int         `json:"percent"`
	StartedAt   string      `json:"started_at,omitempty"`
	CompletedAt string      `json:"completed_at,omitempty"`
	LastAt      string      `json:"last_at,omitempty"` // the last lesson finished
	StageIdx    int         `json:"stage_idx"`         // the stage the next lesson is in
	StageName   string      `json:"stage_name"`
	Next        *lessonView `json:"next,omitempty"`
	Stages      []stageView `json:"stages,omitempty"`
}

// programProgress builds a program's view for one person. Lessons unlock in
// order: a lesson is open when every lesson before it is done; done lessons
// can be repeated any time.
func (s *Server) programProgress(ctx context.Context, uid int64, prog content.Program) (*programView, error) {
	done, err := s.store.LessonsDone(ctx, uid, prog.Key)
	if err != nil {
		return nil, err
	}
	starts, _ := s.store.ProgramStarts(ctx, uid)
	doneAt := map[string]string{}
	last := ""
	for _, d := range done {
		doneAt[d.Lesson] = d.DoneAt
		if d.DoneAt > last {
			last = d.DoneAt
		}
	}
	v := &programView{Key: prog.Key, Name: prog.Name, Kind: prog.Kind, Tagline: prog.Tagline, Description: prog.Description, Color: prog.Color, Icon: prog.Icon, StartedAt: starts[prog.Key]}
	open := true
	for si, st := range prog.Stages {
		sv := stageView{Key: st.Key, Name: st.Name, Tagline: st.Tagline, Total: len(st.Lessons), Locked: !open}
		for _, l := range st.Lessons {
			lv := lessonView{Lesson: l, Program: prog.Key, Stage: st.Key, StageIdx: si, Index: v.Total}
			v.Total++
			sv.Minutes += l.Practice.Minutes
			if at, ok := doneAt[l.Key]; ok {
				lv.Done, lv.DoneAt = true, at
				sv.Done++
				v.Done++
			} else {
				lv.Locked = !open
				if open && v.Next == nil {
					lv.Current = true
					cp := lv
					v.Next = &cp
					v.StageIdx, v.StageName = si, st.Name
				}
				open = false
			}
			sv.Lessons = append(sv.Lessons, lv)
		}
		sv.Complete = sv.Done == sv.Total
		v.Stages = append(v.Stages, sv)
	}
	if v.Total > 0 {
		v.Percent = v.Done * 100 / v.Total
	}
	v.LastAt = last
	if v.Done == v.Total && v.Total > 0 {
		v.CompletedAt = last
		v.StageIdx, v.StageName = len(prog.Stages)-1, prog.Stages[len(prog.Stages)-1].Name
	}
	return v, nil
}

func (s *Server) handleListPrograms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	out := make([]programView, 0, len(content.Programs))
	for _, p := range content.Programs {
		v, err := s.programProgress(ctx, uid, p)
		if err != nil {
			s.fail(w, err, "programs")
			return
		}
		v.Stages = nil
		out = append(out, *v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetProgram(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	prog, ok := content.ProgramByKey(chi.URLParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	v, err := s.programProgress(ctx, actorID(ctx), prog)
	if err != nil {
		s.fail(w, err, "program")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleStartProgram(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	prog, ok := content.ProgramByKey(chi.URLParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.store.StartProgram(ctx, actorID(ctx), prog.Key); err != nil {
		s.fail(w, err, "start")
		return
	}
	s.handleGetProgram(w, r)
}

func (s *Server) handleResetProgram(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	prog, ok := content.ProgramByKey(chi.URLParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.store.ResetProgram(ctx, actorID(ctx), prog.Key); err != nil {
		s.fail(w, err, "reset")
		return
	}
	s.handleGetProgram(w, r)
}

// handleCompleteLesson marks a lesson done without a timed practice (read
// and practised away from the app). Locked lessons stay locked.
func (s *Server) handleCompleteLesson(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	prog, ok := content.ProgramByKey(chi.URLParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	key := chi.URLParam(r, "lesson")
	v, err := s.programProgress(ctx, uid, prog)
	if err != nil {
		s.fail(w, err, "program")
		return
	}
	for _, st := range v.Stages {
		for _, l := range st.Lessons {
			if l.Key != key {
				continue
			}
			if l.Locked {
				writeErr(w, http.StatusConflict, "finish the lessons before this one first")
				return
			}
			if fresh, err := s.store.CompleteLesson(ctx, uid, prog.Key, key, 0); err != nil {
				s.fail(w, err, "complete")
				return
			} else if fresh {
				s.publish(ctx, uid, "still.lesson_completed", prog.Key+"/"+key, map[string]any{"program": prog.Key, "program_name": prog.Name, "lesson": key, "title": l.Title, "stage": l.Stage})
			}
			unlocked := s.checkAchievements(ctx, uid, nil)
			nv, _ := s.programProgress(ctx, uid, prog)
			writeJSON(w, http.StatusOK, map[string]any{"program": nv, "unlocked": unlocked})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "no such lesson")
}

// programByKeyProgress is programProgress by key (nil for an unknown program).
func (s *Server) programByKeyProgress(ctx context.Context, uid int64, key string) (*programView, error) {
	prog, ok := content.ProgramByKey(key)
	if !ok {
		return nil, nil
	}
	return s.programProgress(ctx, uid, prog)
}
