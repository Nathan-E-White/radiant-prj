#!/usr/bin/env bash

printf "%s\n" "Available terminals";
printf "%s\n" "${TERM}" >&1;

git config --global color.ui auto;
git config --global color.status auto;
git config --global color.diff auto;
git config --global color.branch auto;
git config --global color.pager auto;
git config --global color.decorate auto;
git config --global color.grep auto;
git config --global color.interactive auto;
git config --global color.blame auto;
git config --global color.push auto;
git config --global color.remote auto;
git config --global color.showBranch auto;
git config --global color.transport auto;


git config --global color.decorate.branch "bold green"
git config --global color.decorate.remoteBranch "red"
git config --global color.decorate.tag "bold yellow"
git config --global color.decorate.HEAD "bold cyan"

git config --global color.diff.old    "red"
git config --global color.diff.new    "green"
git config --global color.diff.meta   "cyan"
git config --global color.diff.frag   "magenta bold"

git config --global core.pager "less -FRX"
git config --global color.pager true
