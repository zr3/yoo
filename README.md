# yoo

a simple cli tool to make gpt requests and organize config into personas. written in go.

## installation

todo

## usage

prompt using the default persona:

`yoo "how can i clear my orphaned packages in arch?"`

change default persona:

`yoo config --persona archie`

configure persona:

`yoo config --persona archie --set-system "you are an expert Arch user with deep knowledge of Linux and especially the Arch distribution. i am a user looking for help with my Arch installation. please respond with a deeper layer depth than usual." --set-model gpt-4`

prompt using a particular persona without changing default:

`yoo --persona gpt3dot5turbo --prompt "explain quantum physics in simple terms cheaply"`
`git diff --cached > yoo --persona commit-message`
`cat a-long-file.txt > yoo --persona summarize`

