package main

import (
	"fmt"

	"timepointlib/timepoint"
)

//go:generate go run ../cmd/timepointgen -w .

type Account struct {
	Owner string
	Plan  string
}

func main() {
	step := 1
	status := "start"
	account := &Account{Owner: "ana", Plan: "free"}
	values := []int{1, 2, 3}

	// After running go generate, this call is rewritten to include all in-scope variables.
	tp, err := timepoint.Create(
		timepoint.WithName("auto-capture"),
		timepoint.WithProgramCounter("post-checkpoint"),
		timepoint.WithOverrides(map[string]any{
			"status": "captured-by-override",
		}),
		timepoint.WithResume(func(tp *timepoint.Timepoint) error {
			fmt.Println("resume label:", tp.ProgramCounter().Label)
			fmt.Printf("resumed: step=%d status=%q plan=%q values=%v\n", step, status, account.Plan, values)
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}

	step = 99
	status = "changed"
	account.Plan = "enterprise"
	values[0] = 42

	if err := tp.Resume(nil); err != nil {
		panic(err)
	}
}
