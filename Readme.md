# Witty

Witty is a smart terminal emulator, powered by [OpenAI Codex](https://openai.com/blog/openai-codex/).
It will suggest completions for anything run under it (shell, text editors etc.) in a way that is similar to [Github Copilot](https://copilot.github.com/). 

# Demos

In the demos below the autocomplete suggestions are rendered in red. 

## Command-line 

Witty knows how to perform many mundane command-line tasks in different operating systems. Here it suggests
how to handle image conversion:


https://user-images.githubusercontent.com/1808006/143602390-d2ecd65d-7fa0-4952-a630-438467b2b7ca.mov



## System Administration

Here Witty generates configuration changes for the Nginx web server based on user prompts:

https://user-images.githubusercontent.com/1808006/143598850-93696183-8bb2-4992-b45a-16e2836a71c9.mp4

## Git workflow

Here Witty suggests a Git commit message based on the source code diff:


https://user-images.githubusercontent.com/1808006/143599435-c8bd4143-d22f-429b-8e46-72065ac46482.mov


## Data Science

Here Witty manipulates a CSV file and performs data transformations: 

https://user-images.githubusercontent.com/1808006/143598361-e68f450b-6586-4cef-b1e1-0dd89901bf08.mp4

# Credits

This project would not have been possible without:
- [OpenAI Codex](https://openai.com/blog/openai-codex/), of course.
- [vt10x](github.com/ActiveState/vt10x), a terminal emulator backend in Go
- [tcell](github.com/gdamore/tcell), a terminal screen renderer in Go
