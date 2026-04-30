package main

import "github.com/keepmind9/agent-chat/cmd"

func main() {
	cmd.WebFS = WebFS
	cmd.Execute()
}
