# vfsftp [![Go Report Card](https://goreportcard.com/badge/github.com/worldiety/vfsftp)](https://goreportcard.com/report/github.com/worldiety/vfsftp) [![GoDoc](https://godoc.org/github.com/worldiety/vfsftp?status.svg)](http://godoc.org/github.com/worldiety/vfsftp)
A vfs implementation for ftp. This implementation may not work with all ftp servers, because it assumes
that *cwd* is not needed anymore and each command works with absolute paths. This increases the performance
a lot.

## vfsftp.Connect()

`import github.com/worldiety/vfsftp`

| CTS Check     | Result        |
| ------------- | ------------- |
| Empty|:white_check_mark: |
| Write any|:white_check_mark: |
| Read any|:white_check_mark: |
| Write and Read|:white_check_mark: |
| Rename|:white_check_mark: |
| Attributes|:white_check_mark: |
| Close|:white_check_mark: |