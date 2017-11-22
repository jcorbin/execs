package main

import "fmt"

type prompt struct {
	prior  *prompt
	mess   string
	action []promptAction
}

type promptAction struct {
	mess string
	run  func(prompt) (prompt, bool)
}

func (pr prompt) render(prefix string) []string {
	if pr.mess == "" {
		return nil
	}
	lines := make([]string, 0, 1+len(pr.action))
	lines = append(lines, fmt.Sprintf("%s%s: (Press Number, 0 to exit menu)", prefix, pr.mess))
	for i, act := range pr.action {
		lines = append(lines, fmt.Sprintf("%s%d) %s", prefix, i+1, act.mess))
	}
	return lines
}

func (pr *prompt) handle(ch rune) bool {
	if pr.mess == "" {
		return false
	}
	if new, ok := pr.run(ch); ok {
		*pr = new
		return true
	}
	pr.reset()
	return false
}

func (pr *prompt) reset() {
	*pr = pr.unwind()
	pr.mess = ""
	pr.action = pr.action[:0]
}

func (pr *prompt) addAction(
	run func(prompt) (prompt, bool),
	mess string, args ...interface{},
) bool {
	if len(pr.action) < cap(pr.action) {
		pr.action = append(pr.action, promptAction{mess, run})
		return true
	}
	return false
}

func (pr prompt) run(ch rune) (prompt, bool) {
	n := int(ch - '0')
	if n < 0 || n > 9 {
		return pr, false
	}
	if i := n - 1; i < 0 {
		return pr.pop(), true
	} else if i < len(pr.action) {
		return pr.action[i].run(pr)
	}
	return pr, true
}

func (pr prompt) pop() prompt {
	if pr.prior != nil {
		return *pr.prior
	}
	return pr
}

func (pr prompt) unwind() prompt {
	for pr.prior != nil {
		pr = *pr.prior
	}
	return pr
}

func (pr prompt) makeSub(mess string, args ...interface{}) prompt {
	return prompt{
		prior:  &pr,
		mess:   fmt.Sprintf(mess, args...),
		action: make([]promptAction, 0, 10),
	}
}
