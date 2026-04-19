package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/leaanthony/wails-triage-bot/internal/tools"
)

func triageHandler(d *tools.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nStr := r.URL.Query().Get("number")
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			http.Error(w, "number required", http.StatusBadRequest)
			return
		}
		args, _ := json.Marshal(map[string]int{"number": n})
		result, err := d.Dispatch(r.Context(), "check_duplicate", args, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(result)
	}
}

func issueHandler(d *tools.Dispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nStr := r.URL.Query().Get("number")
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			http.Error(w, "number required", http.StatusBadRequest)
			return
		}
		args := []byte(fmt.Sprintf(`{"number":%d}`, n))
		result, err := d.Dispatch(r.Context(), "get_issue", args, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(result)
	}
}
