package main
import (
  "context"
  "fmt"
  "os"
  "time"
  "codex-register/internal/config"
  "codex-register/internal/runtime"
)
func main() {
  cfg, err := config.LoadConfig()
  if err != nil { panic(err) }
  _ = cfg
  settings := runtimeTestSettings()
  for i := 1; i <= 5; i++ {
    ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
    msgs := []string{}
    p := runtime.ResolveRegistrationProxy(ctx, "", settings, func(s string){ msgs = append(msgs, s) })
    cancel()
    fmt.Printf("run=%d proxy=%q\n", i, p)
    for _, m := range msgs { fmt.Println(" log:", m) }
  }
}
func runtimeTestSettings() runtime.EngineSettingsExpose { return runtime.NewEngineSettingsExposeForTest() }
