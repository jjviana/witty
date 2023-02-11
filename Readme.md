# WiTTY

Witty is a smart terminal emulator powered by large code language models. It currently supports [OpenAI Codex](https://openai.com/blog/openai-codex/) and
[Amazon CodeWhisperer](https://aws.amazon.com/codewhisperer/). 

As any terminal emulator, Witty will start the selected shell and pass all input to it. However, every time
the terminal is idle (5 seconds by default), Witty will attempt to generate a completion suggestion. The suggestion
will be rendered in a different color (configurable through the -c argument). Pressing tab will cause the suggestion to be accepted,
and Witty will behave as if the user had typed it. Pressing any other key will cause the suggestion to be discarded. See the Demos section below
for examples.

## Getting Started

To use Codex, you will need an OpenAI API key with access to the Codex models. You can request one  [here](https://beta.openai.com/signup). 
As of this writing, Codex is in private beta and requires an invitation to access.

To use CodeWhisperer, you will be asked the first time you run Witty to log in using your AWS Builder ID and authorize Witty to access CodeWhisperer on your behalf.
As of this writing, CodeWhisperer is in public beta and can be accessed by anyone with a free AWS Builder account.

## Installation

### From source

```
git clone https://github.com/jjviana/witty.git
cd witty/cmd/witty
go build .
./witty 
(see -h for options)
```

### From binary releases

Binaries for MacOS, Linux and Freebsd can be found in the [releases](https://github.com/jjviana/witty/releases) page.

Mac users will need to manually open the app the first time, as it is not signed. To do that,
right-click on the app and select "Open". You will be prompted to confirm that you want to open the app. It will
immediately quit after that, but you will be able to open it normally from now on.

## Running

```
./witty -e codex|codewhisperer [options]
```
The first time it is run with a specific engine, it will ask you to  either
provide an API key (for Codex) or log in with your AWS Builder ID (for CodeWhisperer).

Witty will run your default shell (specified in the SHELL environment variable) unless you specify a different command to run with the -c option.
Arguments after  `--` argument will be passed to the shell.

For instance, to run witty using CodeWhisperer and configuring the shell as a login shell:
```
./witty -e codewhisperer -- --login
```

See `witty -h` for the full list of options.

# Demos

In the demos below the autocomplete suggestions are rendered in red. 

## Command-line 

Witty knows how to perform many mundane command-line tasks in different operating systems. Here it suggests
how to handle image conversion:


https://user-images.githubusercontent.com/1808006/143602390-d2ecd65d-7fa0-4952-a630-438467b2b7ca.mov

## Kubernetes

Kubernetes controlled from natural language:

https://user-images.githubusercontent.com/1808006/143656487-5d2028fc-2926-46ad-a261-403e763991c6.mov



## System Administration

Here Witty generates configuration changes for the Nginx web server based on user prompts:

https://user-images.githubusercontent.com/1808006/143598850-93696183-8bb2-4992-b45a-16e2836a71c9.mp4

## Git workflow

Here Witty suggests a Git commit message based on the source code diff:


https://user-images.githubusercontent.com/1808006/143599435-c8bd4143-d22f-429b-8e46-72065ac46482.mov


## Data Science

Here Witty manipulates a CSV file and performs data transformations, while seamlessly switching from bash to Python:

https://user-images.githubusercontent.com/1808006/143598361-e68f450b-6586-4cef-b1e1-0dd89901bf08.mp4

## Databases

Generating a SQL query based on table descriptions and prompt: 

https://user-images.githubusercontent.com/1808006/144070103-95d712bc-d266-4ea3-a0b1-0d65f73294c5.mp4




# Credits

This project would not have been possible without:
- [OpenAI Codex](https://openai.com/blog/openai-codex/)
- [Amazon CodeWhisperer](https://aws.amazon.com/codewhisperer/)
- [vt10x](https://github.com/ActiveState/vt10x), a terminal emulator backend in Go
- [tcell](https://github.com/gdamore/tcell), a terminal screen renderer in Go
