// G.2: measure where the KV cache leaves the GPU, on whatever instance is in use.
//
// **The failure this exists to catch is silent.** Ollama sizes the KV cache up front
// from `OLLAMA_CONTEXT_LENGTH`, against total VRAM *plus* total host RAM. When the cache
// no longer fits in VRAM the model still loads and still answers; it just answers at
// host-RAM speed. Nothing logs, nothing errors, and the only symptom is throughput -
// which is why `infer_server_readme.md` carries a measured table rather than a formula,
// and why criterion 25 asks for numbers from the instance actually in use.
//
// **`/api/ps` is the detector, and it is exact.** It reports `size` and `size_vram`
// separately, so `size - size_vram` is the number of bytes that went to host RAM. That
// replaces inference from throughput with a direct reading; throughput is measured too,
// but as the *consequence*, not the evidence.
//
//	go run ./tools/infer_capacity -model granite4.1:3b
//	go run ./tools/infer_capacity -model granite4.1:3b -ctx 32768,49152,57344,61440
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type psModel struct {
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	SizeVRAM      int64  `json:"size_vram"`
	ContextLength int    `json:"context_length"`
}

type genResponse struct {
	EvalCount    int   `json:"eval_count"`
	EvalDuration int64 `json:"eval_duration"`
}

type row struct {
	Ctx      int     `json:"num_ctx"`
	Size     int64   `json:"size"`
	VRAM     int64   `json:"size_vram"`
	Host     int64   `json:"host"`
	TokPerSe float64 `json:"tokens_per_second"`
	Err      string  `json:"error,omitempty"`
}

func main() {
	host := flag.String("host", "http://localhost:11434", "infer server")
	model := flag.String("model", "granite4.1:3b", "model to load")
	ctxList := flag.String("ctx", "8192,16384,32768,49152,57344,61440,65536,98304,131072",
		"context lengths to try, smallest first")
	predict := flag.Int("predict", 64, "tokens to generate, for the throughput reading")
	out := flag.String("out", "", "also write the rows as JSON here")
	flag.Parse()

	var rows []row
	fmt.Printf("%8s %9s %9s %9s %8s  %s\n", "num_ctx", "size GiB", "vram GiB", "host GiB", "tok/s", "state")
	for _, s := range strings.Split(*ctxList, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		r := measure(*host, *model, n, *predict)
		rows = append(rows, r)
		if r.Err != "" {
			fmt.Printf("%8d %9s %9s %9s %8s  load failed: %s\n", n, "-", "-", "-", "-", r.Err)
			continue
		}
		state := "GPU-resident"
		if r.Host > 0 {
			state = fmt.Sprintf("SPILLED %.2f GiB to host", gib(r.Host))
		}
		fmt.Printf("%8d %9.2f %9.2f %9.2f %8.1f  %s\n",
			n, gib(r.Size), gib(r.VRAM), gib(r.Host), r.TokPerSe, state)
	}
	summarise(rows)

	if *out != "" {
		b, _ := json.MarshalIndent(rows, "", "  ")
		if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("\nwrote %s\n", *out)
	}
}

func gib(n int64) float64 { return float64(n) / (1 << 30) }

// measure unloads, reloads at one context length, and reads what actually landed on the
// GPU. The unload matters: Ollama keeps a model resident with its *previous* cache size,
// so measuring without it reads the last run rather than this one.
func measure(host, model string, ctx, predict int) row {
	r := row{Ctx: ctx}
	post(host, "/api/generate", map[string]any{"model": model, "keep_alive": 0}, 60*time.Second)
	time.Sleep(2 * time.Second)

	body, err := post(host, "/api/generate", map[string]any{
		"model": model, "prompt": "Count to twenty.", "stream": false,
		"options": map[string]any{"num_predict": predict, "num_ctx": ctx, "temperature": 0},
	}, 15*time.Minute)
	if err != nil {
		r.Err = short(err.Error())
		return r
	}
	var gen genResponse
	_ = json.Unmarshal(body, &gen)
	if gen.EvalDuration > 0 {
		r.TokPerSe = float64(gen.EvalCount) / (float64(gen.EvalDuration) / 1e9)
	}

	ps, err := post(host, "/api/ps", nil, 30*time.Second)
	if err != nil {
		r.Err = short(err.Error())
		return r
	}
	var loaded struct {
		Models []psModel `json:"models"`
	}
	_ = json.Unmarshal(ps, &loaded)
	for _, m := range loaded.Models {
		if strings.HasPrefix(m.Name, strings.SplitN(model, ":", 2)[0]) {
			r.Size, r.VRAM = m.Size, m.SizeVRAM
			if h := m.Size - m.SizeVRAM; h > 0 {
				r.Host = h
			}
			return r
		}
	}
	r.Err = "model not resident after generate"
	return r
}

// summarise brackets the spill point, which is the number the readme and the
// OLLAMA_CONTEXT_LENGTH comment both quote. **Bracketed, not interpolated** - the sweep
// knows the largest context that stayed resident and the smallest that did not, and
// anything between them is untested.
func summarise(rows []row) {
	var lastOK, firstSpill *row
	var baseline float64
	for i := range rows {
		r := &rows[i]
		if r.Err != "" {
			continue
		}
		if r.Host == 0 {
			lastOK = r
			if r.TokPerSe > baseline {
				baseline = r.TokPerSe
			}
		} else if firstSpill == nil {
			firstSpill = r
		}
	}
	fmt.Println()
	switch {
	case lastOK == nil:
		fmt.Println("no context length stayed GPU-resident")
	case firstSpill == nil:
		fmt.Printf("no spill observed up to %d\n", lastOK.Ctx)
	default:
		fmt.Printf("spill point is between %d and %d\n", lastOK.Ctx, firstSpill.Ctx)
		if baseline > 0 {
			fmt.Printf("cost of spilling %.2f GiB: %.1f -> %.1f tok/s (%.0f%% of resident speed)\n",
				gib(firstSpill.Host), baseline, firstSpill.TokPerSe,
				100*firstSpill.TokPerSe/baseline)
		}
	}
	if lastOK != nil {
		perToken := float64(lastOK.Size) / float64(lastOK.Ctx) / (1 << 20)
		fmt.Printf("at %d: %.2f GiB total, ~%.2f MiB per token including the base\n",
			lastOK.Ctx, gib(lastOK.Size), perToken)
	}
}

func post(host, path string, payload any, timeout time.Duration) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	method := http.MethodPost
	if payload == nil && path == "/api/ps" {
		method = http.MethodGet
	}
	req, err := http.NewRequest(method, host+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, short(string(out)))
	}
	return out, nil
}

func short(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 90 {
		return s[:90]
	}
	return s
}
