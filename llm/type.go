package llm

type LLM interface {
	Ask(question []string) (string, error)
}
