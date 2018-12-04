# vfs-ftp [![Go Report Card](https://goreportcard.com/badge/github.com/worldiety/vfs-ftp)](https://goreportcard.com/report/github.com/worldiety/vfs-ftp) [![GoDoc](https://godoc.org/github.com/worldiety/vfs-ftp?status.svg)](http://godoc.org/github.com/worldiety/vfs-ftp)
A vfs implementation for ftp. This implementation may not work with all ftp servers, because it assumes
that *cwd* is not needed anymore and each command works with absolute paths. This increases the performance
a lot.

## ftp.Connect()

`import github.com/worldiety/vfs-ftp`

| CTS Check     | Result        |
| ------------- | ------------- |
| Empty|:white_check_mark: |
| Write any|:white_check_mark: |
| Read any|:white_check_mark: |
| Write and Read|:white_check_mark: |
| Rename|:white_check_mark: |
| Attributes|:white_check_mark: |
| Close|:white_check_mark: |