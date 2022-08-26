# file sorter

A simple script that moves files into folder given the year & month.

Sorted 6k files (50GB) in 2 seconds

## Assumptions

You have a folder with an insane amount of files
You want to sort files per year & month in the same folder.

Files follow one of the following naming formats: `IMG_YYYYMMDD_*+` or `YYYYMMDD_*+`

You are ok ignoring folders


## disclaimer

Was lazy so the executor is from https://github.com/godoylucase/workers-pool

Given that multiple threads will try to create folders at the same time is expected to see some errors creating folders but works as expected