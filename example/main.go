package main

import (
	"fmt"

	"timepointlib/timepoint"
)

type Session struct {
	User  string
	Quota int
}

func main() {
	// Variables currently in scope.
	step := 1                    // stack-like value
	message := "initial"         // stack-like value
	session := &Session{User: "ana", Quota: 3} // heap-like value
	items := []string{"A", "B"} // heap-like value

	p, err := timepoint.Create(
		timepoint.WithName("checkpoint-login"),
		timepoint.WithProgramCounter("after-auth"),
		timepoint.WithVariables(
			timepoint.StackVar("step", &step),
			timepoint.StackVar("message", &message),
			timepoint.HeapVar("session", &session),
			timepoint.HeapVar("items", &items),
		),
		timepoint.WithOverrides(map[string]any{
			"message": "snapshotted-value", // override value saved in the snapshot
		}),
		timepoint.WithResume(func(tp *timepoint.Timepoint) error {
			fmt.Println("resume callback running from", tp.ProgramCounter().Label)
			fmt.Printf("state in resume: step=%d message=%q quota=%d items=%v\n", step, message, session.Quota, items)
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(p.ToString())

	// Mutate everything.
	step = 99
	message = "changed"
	session.Quota = 0
	items = append(items, "C")
	fmt.Printf("after mutation: step=%d message=%q quota=%d items=%v\n", step, message, session.Quota, items)

	// Restore only stack variables.
	if err := p.RestoreStack(map[string]any{"step": 7}); err != nil {
		panic(err)
	}
	fmt.Printf("after RestoreStack: step=%d message=%q quota=%d items=%v\n", step, message, session.Quota, items)

	// Restore only heap variables.
	if err := p.RestoreHeap(nil); err != nil {
		panic(err)
	}
	fmt.Printf("after RestoreHeap: step=%d message=%q quota=%d items=%v\n", step, message, session.Quota, items)

	// Full restore + continuation callback. Override message at resume time.
	if err := p.Resume(map[string]any{"message": "resume-override"}); err != nil {
		panic(err)
	}
}
