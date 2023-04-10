# yoo

a sleek cli tool for gpt interactions. easily create and switch personas. made with go.

Yet Onother OpenAI cli tool (jk)

## installation

build with go and add `yoo` to `~/.local/bin` or wherever you like to keep things

add `~/.config/yoo/config.yml`:
```yaml
secrets:
  openai-key: YOUR_KEY_HERE
default-persona: default
  - name: default
    model: gpt-4
```

add `~/.config/yoo/default.txt`:
```
you are an ai chatbot with deep knowledge of Arch. I am looking for help and will ask you a question. please be detailed in your response. accuracy is more important than precision. please point me to places where I can further read when appropriate.
```

## usage

prompt using the default persona:

`yoo "how can i clear my orphaned packages in arch?"`

(wip) change default persona:

`yoo config --persona archie`

(wip) configure persona:

`yoo config --persona archie --set-system "you are an expert Arch user with deep knowledge of Linux and especially the Arch distribution. i am a user looking for help with my Arch installation. please respond with a deeper layer depth than usual." --set-model gpt-4`

prompt using a particular persona without changing default:

`yoo --persona gpt3dot5turbo --prompt "explain quantum physics in simple terms cheaply"`
`git diff --cached > yoo --persona commit-message`
`cat a-long-file.txt > yoo --persona summarize`

