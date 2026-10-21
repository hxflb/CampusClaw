package skillfile

import "strings"

const Example = "名称：数学解题引导\n适用条件：方程,去括号,分数,通分,应用题\n资料：数学\n工具：knowledge_search\n启用：是\n\n学生正在自己做数学作业，往往已经写了一步。先看懂他卡在哪一步，只指出下一步该检查什么。不要算出最终得数，不要把整道题做完。依据只用数学自学笔记。\n"

type Doc struct {
	Name    string
	Trigger string
	Subject string
	Tool    string
	Enabled bool
	Body    string
	Source  string
}

func ValidTool(tool string) bool {
	if tool == "knowledge_search" {
		return true
	}
	name, server, ok := strings.Cut(tool, "@")
	if !ok || name == "" || server == "" || strings.Contains(name, "@") {
		return false
	}
	if name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	if !serverName(server) {
		return false
	}
	return len([]rune(tool)) <= 64
}

func serverName(server string) bool {
	if server == "" {
		return false
	}
	for i, r := range server {
		letter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		digit := r >= '0' && r <= '9'
		if i == 0 && !letter {
			return false
		}
		if !letter && !digit && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func Parse(source string) (Doc, bool) {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	source = strings.TrimSpace(source)
	if source == "" || len([]rune(source)) > 8000 {
		return Doc{}, false
	}
	head, body, ok := strings.Cut(source, "\n\n")
	if !ok {
		return Doc{}, false
	}
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 2000 {
		return Doc{}, false
	}
	fields := map[string]string{}
	allowed := map[string]bool{"名称": true, "适用条件": true, "资料": true, "工具": true, "启用": true}
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		label, value, found := strings.Cut(line, "：")
		if !found || !allowed[label] {
			return Doc{}, false
		}
		if _, exists := fields[label]; exists {
			return Doc{}, false
		}
		fields[label] = strings.TrimSpace(value)
	}
	name := fields["名称"]
	trigger := fields["适用条件"]
	subject := fields["资料"]
	tool := fields["工具"]
	flag := fields["启用"]
	if _, ok := fields["名称"]; !ok {
		return Doc{}, false
	}
	if _, ok := fields["适用条件"]; !ok {
		return Doc{}, false
	}
	if _, ok := fields["资料"]; !ok {
		return Doc{}, false
	}
	if _, ok := fields["工具"]; !ok {
		return Doc{}, false
	}
	if _, ok := fields["启用"]; !ok {
		return Doc{}, false
	}
	if name == "" || len([]rune(name)) > 32 || len([]rune(trigger)) > 120 || len([]rune(subject)) > 32 {
		return Doc{}, false
	}
	if !ValidTool(tool) || (flag != "是" && flag != "否") {
		return Doc{}, false
	}
	return Doc{
		Name: name, Trigger: trigger, Subject: subject, Tool: tool,
		Enabled: flag == "是", Body: body, Source: source + "\n",
	}, true
}
