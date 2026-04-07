package main

import "github.com/spf13/cobra"

// runTemplateBranch is the hook that 05-02's apply command RunE calls when
// --template is set. 05-03 (this plan) assigns it to applyTemplateRun in
// cmd_apply_template.go's init(). Declared here so that both 05-02 and 05-03
// can be developed in parallel without importing each other's files. If 05-02
// lands first and declares this var, remove this file.
var runTemplateBranch func(cmd *cobra.Command, args []string) error
