<h1 align="center">gitlab-subdomains</h1>

<h4 align="center">Find subdomains on GitLab.</h4>

<p align="center">
    <img src="https://img.shields.io/badge/go-v1.13-blue" alt="go badge">
    <img src="https://img.shields.io/badge/license-MIT-green" alt="MIT license badge">
    <a href="https://twitter.com/intent/tweet?text=https%3a%2f%2fgithub.com%2fgwen001%2fgitlab-subdomains%2f" target="_blank"><img src="https://img.shields.io/twitter/url?style=social&url=https%3A%2F%2Fgithub.com%2Fgwen001%2Fgitlab-subdomains" alt="twitter badge"></a>
</p>

<!-- <p align="center">
    <img src="https://img.shields.io/github/stars/gwen001/gitlab-subdomains?style=social" alt="github stars badge">
    <img src="https://img.shields.io/github/watchers/gwen001/gitlab-subdomains?style=social" alt="github watchers badge">
    <img src="https://img.shields.io/github/forks/gwen001/gitlab-subdomains?style=social" alt="github forks badge">
</p> -->

---

## Description

This Go tool performs searches on GitLab and parses the results to find subdomains of a given domain.

## Important note

‼ GitLab search is very limited ‼
Check the [official documentation](https://docs.gitlab.com/ee/api/search.html) for more information.

## Requirements

You need a GitLab token, if you don't have any you can easily create a free account on [gitlab.com](https://gitlab.com/) or use my [github-regexp](https://github.com/gwen001/github-regexp) to find one or more...

## Install

```
go install github.com/gwen001/gitlab-subdomains@latest
```

or

```
git clone https://github.com/gwen001/gitlab-subdomains
cd gitlab-subdomains
go install
```

## Usage

```
$ gitlab-subdomains -h

Usage of gitlab-subdomains:
  -d string
        domain you are looking for (required)
  -debug
        debug mode
  -e    extended mode, also look for <dummy>example.com
  -o string
        output file name
  -t string
        gitlab token (required), can be:
          • a single token
          • a list of tokens separated by comma
          • a file (.tokens) containing 1 token per line
        if the options is not provided, the environment variable GITLAB_TOKEN is readed, it can be:
          • a single token
          • a list of tokens separated by comma
```
```bash
gitlab-subdomains -d hackerone.com -t "*****-i*-*************4***" -o subdomains.txt
```
```bash
gitlab-subdomains -d hackerone.com -t "*****-i*-*************4***" | (tee/anew) subdomains.txt
```

If you want to use multiple tokens, you better create a `.tokens` file in the executable directory with 1 token per line  
```
token1
token2
...
```
or use an environment variable with tokens separated by comma:  
```
export GITLAB_TOKEN=token1,token2...
```

Tokens are disabled when GitLab raises a rate limit alert, however they are re-enable 1mn later.
You can disable that feature by using the option `-k`.

---

<img src="https://raw.githubusercontent.com/gwen001/gitlab-subdomains/master/preview.gif">

---

Feel free to [open an issue](/../../issues/) if you have any problem with the script.  

