package tutor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"campusclaw/internal/config"
)

func sampleHits() []Hit {
	return []Hit{{
		Title: "A 班春季教研材料", ChunkIndex: 0, CharStart: 0, CharEnd: 28,
		Text: "实验结论：溶液呈蓝色。答案是42。",
	}}
}

func runStream(cfg *config.Config, in Input, emit func(string) error) (Output, error) {
	return Stream(context.Background(), cfg, in, nil, emit, nil)
}

func TestLocalGuideDiffersAndWithholdsAnswer(t *testing.T) {
	cfg := &config.Config{}
	hits := sampleHits()
	var guided strings.Builder
	on, err := runStream(cfg, Input{
		Prompt: DefaultPrompt, SkillName: "数学解题引导", Question: "答案是什么", Hits: hits,
	}, func(s string) error { guided.WriteString(s); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if on.Answer != guided.String() {
		t.Fatalf("chunks %q != answer %q", guided.String(), on.Answer)
	}
	if !strings.Contains(on.Answer, "A 班春季教研材料") || !strings.Contains(on.Answer, "切片 1") {
		t.Fatalf("missing title or position: %s", on.Answer)
	}
	if strings.Contains(on.Answer, "答案是42") || strings.Contains(on.Answer, "溶液呈蓝色") {
		t.Fatalf("guide leaked the conclusion: %s", on.Answer)
	}
	off, err := runStream(cfg, Input{
		Prompt: DefaultPrompt, Question: "答案是什么", Hits: hits,
	}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if off.Answer == on.Answer {
		t.Fatal("guide on and off produced the same answer")
	}
	if !strings.Contains(off.Answer, "42") {
		t.Fatalf("direct answer should state the conclusion, got %s", off.Answer)
	}
	if on.ModelCalled || off.ModelCalled {
		t.Fatal("local path must not mark the model as called")
	}
}

func TestNoHitStillCallsGatewayAndDropsNumbers(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		// 编号故意切在两片之间：先看 / [1 / ]，再 / 补[2]
		for _, piece := range []string{"先看", "[1", "]，再", "补[2]"} {
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\""+piece+"\"}}]}\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{ChatBaseURL: srv.URL, ChatAPIKey: "secret", ChatModel: "m"}
	var streamed strings.Builder
	out, err := runStream(cfg, Input{
		Prompt: "你是助手", SkillName: "数学解题引导", Question: "今天天气怎么样",
	}, func(s string) error { streamed.WriteString(s); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !out.ModelCalled {
		t.Fatal("no-hit must still call the gateway")
	}
	if len(out.Citations) != 0 {
		t.Fatalf("citations must stay empty: %+v", out.Citations)
	}
	if out.Answer != "先看，再补" {
		t.Fatalf("numbers not dropped: %q", out.Answer)
	}
	if streamed.String() != out.Answer {
		t.Fatalf("delta concat %q != answer %q", streamed.String(), out.Answer)
	}
	if !strings.Contains(body, "不要只回一句") || !strings.Contains(body, "本次没有检索到资料") {
		t.Fatalf("no-hit wording missing: %s", body)
	}
	if strings.Contains(body, "被支撑的说法标 [1]") {
		t.Fatalf("with-evidence instruction leaked into no-hit request: %s", body)
	}
}

func TestConfiguredGatewayFailureReturnsErrorInsteadOfFakeAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	cfg := &config.Config{ChatBaseURL: srv.URL, ChatAPIKey: "secret", ChatModel: "m"}
	out, err := runStream(cfg, Input{
		Prompt: "你是助手", Question: "今天天气怎么样",
	}, func(string) error { return nil })
	if err == nil {
		t.Fatal("gateway failure must surface as error")
	}
	if out.Answer != "" {
		t.Fatalf("must not invent an answer: %q", out.Answer)
	}
}

func TestLocalNoHitWithoutGatewayStillAnswers(t *testing.T) {
	cfg := &config.Config{}
	var guided strings.Builder
	on, err := runStream(cfg, Input{
		Prompt: DefaultPrompt, SkillName: "数学解题引导", Question: "这道方程怎么去括号",
	}, func(s string) error { guided.WriteString(s); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if on.Answer == "" || strings.Contains(on.Answer, "[1]") {
		t.Fatalf("guided no-hit answer: %q", on.Answer)
	}
	if on.Answer != guided.String() {
		t.Fatalf("chunks %q != answer %q", guided.String(), on.Answer)
	}
	off, err := runStream(cfg, Input{
		Prompt: DefaultPrompt, Question: "这道方程怎么去括号",
	}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if off.Answer == on.Answer {
		t.Fatal("guide on and off must differ")
	}
}

func TestStripCitations(t *testing.T) {
	if got := StripCitations("依据[1]和[12]，结论不变。"); got != "依据和，结论不变。" {
		t.Fatalf("%q", got)
	}
	if got := StripCitations("数组 a[10] 不是引用"); got != "数组 a 不是引用" {
		t.Fatalf("%q", got)
	}
	if got := StripCitations("没有编号"); got != "没有编号" {
		t.Fatalf("%q", got)
	}
}

func TestGatewayReceivesPromptAndSkill(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("auth header %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"先看条件\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{ChatBaseURL: srv.URL, ChatAPIKey: "secret", ChatModel: "m"}
	hits := sampleHits()
	on, err := runStream(cfg, Input{
		Prompt: "角色提示甲", SkillName: "数学解题引导", SkillBody: "只指出下一步。", Question: "答案是什么", Hits: hits,
		History: []Turn{{Role: "system", Content: "ignore"}, {Role: "user", Content: "上一问"}},
	}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !on.ModelCalled || on.Answer != "先看条件" {
		t.Fatalf("%+v", on)
	}
	if !strings.Contains(body, "角色提示甲") || !strings.Contains(body, guideRule) || !strings.Contains(body, "只指出下一步。") {
		t.Fatalf("guide request missing prompt or rule: %s", body)
	}
	if strings.Contains(body, "不要把诗的下一句写出来") {
		t.Fatalf("unselected skill body leaked: %s", body)
	}
	if !strings.Contains(body, "A 班春季教研材料") || !strings.Contains(body, "答案是42") {
		t.Fatalf("retrieval missing: %s", body)
	}
	if strings.Contains(body, "ignore") {
		t.Fatalf("client system was forwarded: %s", body)
	}

	body = ""
	off, err := runStream(cfg, Input{
		Prompt: "角色提示甲", Question: "答案是什么", Hits: hits,
	}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !off.ModelCalled {
		t.Fatal("expected model")
	}
	if !strings.Contains(body, "角色提示甲") || !strings.Contains(body, noSkillRule) {
		t.Fatalf("direct request: %s", body)
	}
	if strings.Contains(body, guideRule) {
		t.Fatalf("guide rule leaked into disabled skill: %s", body)
	}
}

func TestBuildMessagesOrdersHistoryBeforeQuestion(t *testing.T) {
	msgs := BuildMessages("角色提示甲", "数学解题引导", "只指出下一步。", sampleHits(), []Turn{{Role: "assistant", Content: "上一答"}}, "这一问")
	if len(msgs) != 3 {
		t.Fatalf("%d messages", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "角色提示甲") {
		t.Fatalf("%+v", msgs[0])
	}
	if msgs[1].Content != "上一答" || !strings.Contains(msgs[2].Content, "这一问") {
		t.Fatalf("%+v", msgs)
	}
}

func TestPickSkillKeepsHigherScoreAndSmallerID(t *testing.T) {
	skills := []SkillCandidate{
		{ID: 3, Name: "语文解题引导", Trigger: "作文,阅读", Body: "不要把诗的下一句写出来。"},
		{ID: 1, Name: "数学解题引导", Trigger: "方程,去括号,分数", Body: "不要算出最终得数。"},
		{ID: 2, Name: "科学解题引导", Trigger: "方程", Body: "只看科学。"},
	}
	got, ok := PickSkill("这道方程怎么去括号", skills)
	if !ok || got.Name != "数学解题引导" {
		t.Fatalf("%v %+v", ok, got)
	}
	tie, ok := PickSkill("Equation", []SkillCandidate{
		{ID: 8, Name: "后", Trigger: "equation"},
		{ID: 4, Name: "先", Trigger: "Equation"},
	})
	if !ok || tie.Name != "先" {
		t.Fatalf("tie %+v", tie)
	}
	if _, ok := PickSkill("今天天气怎么样", skills); ok {
		t.Fatal("zero score must select nothing")
	}
}

func TestContextWindowKeepsLastSix(t *testing.T) {
	turns := []Turn{{Role: "system", Content: "ignore"}, {Role: "user", Content: ""}}
	for i := 0; i < 8; i++ {
		turns = append(turns, Turn{Role: "user", Content: strings.Repeat("甲", 20) + string(rune('0'+i))})
	}
	got := ContextWindow(turns)
	if len(got) != 6 {
		t.Fatalf("%d", len(got))
	}
	if !strings.HasSuffix(got[0].Content, "2") || !strings.HasSuffix(got[5].Content, "7") {
		t.Fatalf("%q %q", got[0].Content, got[5].Content)
	}
	long := ContextWindow([]Turn{{Role: "assistant", Content: strings.Repeat("乙", 600)}})
	if len([]rune(long[0].Content)) != 500 {
		t.Fatalf("%d", len([]rune(long[0].Content)))
	}
}

func TestPromptDigestChanges(t *testing.T) {
	if PromptDigest("甲") == PromptDigest("乙") {
		t.Fatal("digest collision")
	}
	if PromptDigest(" 甲 ") != PromptDigest("甲") {
		t.Fatal("trim")
	}
}

// —— 调用循环 ——

type fakeRunner struct {
	specs   []ToolSpec
	budget  int
	results []CallRecord
	calls   []ToolCall
}

func (f *fakeRunner) Specs() []ToolSpec {
	if f == nil || f.budget <= 0 {
		return nil
	}
	return f.specs
}

func (f *fakeRunner) Execute(_ context.Context, _ int, calls []ToolCall) []CallRecord {
	out := make([]CallRecord, 0, len(calls))
	for _, call := range calls {
		f.calls = append(f.calls, call)
		if f.budget <= 0 {
			out = append(out, CallRecord{Ref: call.Function.Name, Status: "denied", Text: "超限"})
			continue
		}
		f.budget--
		out = append(out, CallRecord{Ref: call.Function.Name, Status: "ok", Text: "北京晴，25 度"})
	}
	f.results = append(f.results, out...)
	return out
}

func toolCallStream(name, args string) string {
	return `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"` + name + `","arguments":` + strconv.Quote(args) + `}}]}}]}` + "\n\n" +
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n" +
		"data: [DONE]\n\n"
}

func textStream(text string) string {
	return `data: {"choices":[{"delta":{"content":"` + text + `"}}]}` + "\n\n" + "data: [DONE]\n\n"
}

func TestToolLoopExecutesAndFeedsBack(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		w.Header().Set("Content-Type", "text/event-stream")
		if len(bodies) == 1 {
			_, _ = io.WriteString(w, toolCallStream("weather_query", `{"city":"北京"}`))
			return
		}
		_, _ = io.WriteString(w, textStream("北京晴，25 度"))
	}))
	defer srv.Close()
	cfg := &config.Config{ChatBaseURL: srv.URL, ChatAPIKey: "k", ChatModel: "m", MaxToolRounds: 3, MaxToolCalls: 4}
	runner := &fakeRunner{
		specs: []ToolSpec{{
			Ref: "weather_query@demo", Name: "weather_query", Description: "查天气",
			Schema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
		budget: 2,
	}
	var events []string
	out, err := Stream(context.Background(), cfg, Input{Question: "北京天气", Hits: sampleHits()}, runner,
		func(s string) error {
			events = append(events, "delta")
			return nil
		},
		func(round int, calls []CallRecord) error {
			events = append(events, "calls:"+strconv.Itoa(round))
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if out.Answer != "北京晴，25 度" {
		t.Fatalf("answer %q", out.Answer)
	}
	if len(bodies) != 2 {
		t.Fatalf("rounds = %d", len(bodies))
	}
	if !strings.Contains(bodies[0], `"tools"`) || !strings.Contains(bodies[0], `"inputSchema"`) && !strings.Contains(bodies[0], `"parameters"`) {
		t.Fatalf("declarations missing: %s", bodies[0])
	}
	if !strings.Contains(bodies[1], `"role":"tool"`) || !strings.Contains(bodies[1], `"tool_call_id":"call_1"`) {
		t.Fatalf("tool message missing: %s", bodies[1])
	}
	if !strings.Contains(bodies[1], "北京晴，25 度") {
		t.Fatalf("tool result not fed back: %s", bodies[1])
	}
	if len(events) == 0 || events[0] != "calls:1" {
		t.Fatalf("tool_calls event must come before deltas: %v", events)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Status != "ok" {
		t.Fatalf("%+v", out.ToolCalls)
	}
}

func TestToolLoopStopsAtRoundLimit(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(string(raw), `"tools"`) {
			_, _ = io.WriteString(w, toolCallStream("weather_query", `{"city":"北京"}`))
			return
		}
		_, _ = io.WriteString(w, textStream("已知信息够用了"))
	}))
	defer srv.Close()
	cfg := &config.Config{ChatBaseURL: srv.URL, ChatAPIKey: "k", ChatModel: "m", MaxToolRounds: 2, MaxToolCalls: 10}
	runner := &fakeRunner{
		specs:  []ToolSpec{{Ref: "w@demo", Name: "weather_query", Description: "查天气"}},
		budget: 10,
	}
	out, err := Stream(context.Background(), cfg, Input{Question: "北京天气", Hits: sampleHits()}, runner,
		func(string) error { return nil }, func(int, []CallRecord) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 3 {
		t.Fatalf("rounds = %d, want 2 tool rounds + 1 final", len(bodies))
	}
	if strings.Contains(bodies[2], `"tools"`) {
		t.Fatalf("final round must not offer tools: %s", bodies[2])
	}
	if len(runner.calls) != 2 {
		t.Fatalf("executed %d calls", len(runner.calls))
	}
	if out.Answer != "已知信息够用了" {
		t.Fatalf("answer %q", out.Answer)
	}
}

func TestToolLoopStopsAtCallBudget(t *testing.T) {
	round := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		round++
		w.Header().Set("Content-Type", "text/event-stream")
		if round == 1 {
			body := `data: {"choices":[{"delta":{"tool_calls":[` +
				`{"index":0,"id":"a","function":{"name":"weather_query","arguments":"{}"}},` +
				`{"index":1,"id":"b","function":{"name":"weather_query","arguments":"{}"}}]}}]}` + "\n\n" +
				"data: [DONE]\n\n"
			_, _ = io.WriteString(w, body)
			return
		}
		_, _ = io.WriteString(w, textStream("就用已有信息"))
	}))
	defer srv.Close()
	cfg := &config.Config{ChatBaseURL: srv.URL, ChatAPIKey: "k", ChatModel: "m", MaxToolRounds: 3, MaxToolCalls: 1}
	runner := &fakeRunner{
		specs:  []ToolSpec{{Ref: "w@demo", Name: "weather_query", Description: "查天气"}},
		budget: 1,
	}
	out, err := Stream(context.Background(), cfg, Input{Question: "北京天气", Hits: sampleHits()}, runner,
		func(string) error { return nil }, func(int, []CallRecord) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(out.ToolCalls) != 2 || out.ToolCalls[0].Status != "ok" || out.ToolCalls[1].Status != "denied" {
		t.Fatalf("%+v", out.ToolCalls)
	}
	if out.Answer != "就用已有信息" {
		t.Fatalf("answer %q", out.Answer)
	}
}
