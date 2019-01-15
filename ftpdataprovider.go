package vfsftp

import (
	"github.com/jlaffaye/ftp"
	"github.com/worldiety/vfs"
	"io"
	"io/ioutil"
	"net/textproto"
	"net/url"
	"os"
)

var _ vfs.FileSystem = (*ftpDataProvider)(nil)

type ftpDataProvider struct {
	conn *ftp.ServerConn
	// TmpDir Used to store files before uploading. See also ioutil.TempDir. Keep empty to pick the system default.
	TmpDir   string
	myTmpDir string
}

// Connect opens the ftp and performs a login using information from the url.
// This FTP implementation is NOT thread safe, because it only ever uses a single connection which is
// stateful.
func Connect(url *url.URL) (vfs.FileSystem, error) {
	adr := url.Host
	login := url.User.Username()
	password, _ := url.User.Password()

	conn, err := ftp.Connect(adr)
	if err != nil {
		return nil, err
	}
	err = conn.Login(login, password)
	if err != nil {
		return nil, err
	}
	return &ftpDataProvider{conn, "", ""}, nil
}

// Resolve creates a platform specific filename from the given invariant path by adding the Prefix and using
// the platform specific name separator. If AllowRelativePaths is false (default), .. will be silently ignored.
func (dp *ftpDataProvider) Resolve(path vfs.Path) string {
	return path.String()
}

// Open details: see vfs.DataProvider#Open
func (dp *ftpDataProvider) Open(path vfs.Path, flag int, perm os.FileMode) (vfs.Resource, error) {
	if flag == os.O_RDONLY {
		res, err := dp.conn.Retr(dp.Resolve(path))
		if err != nil {
			return nil, wrapErr(err)
		}
		return vfs.NewResourceFromReader(res), nil

	} else {
		if dp.myTmpDir == "" {
			dir, err := ioutil.TempDir(dp.TmpDir, "ftp")
			if err != nil {
				return nil, err
			}
			dp.myTmpDir = dir
		}

		f, err := ioutil.TempFile(dp.myTmpDir, "upload")
		if err != nil {
			return nil, err
		}
		return &diskBufferedWriter{f, dp, dp.Resolve(path)}, nil
	}
}

// Delete details: see vfs.DataProvider#Delete
func (dp *ftpDataProvider) Delete(path vfs.Path) error {
	err := dp.conn.Delete(dp.Resolve(path))
	if err != nil {
		err2 := dp.conn.RemoveDirRecur(dp.Resolve(path))
		if err2 != nil {
			_, err3 := dp.conn.FileSize(dp.Resolve(path))
			if protoErr, ok := err3.(*textproto.Error); ok {
				if protoErr.Code == ftp.StatusFileUnavailable {
					return nil
				}
			}
		}
		return nil
	}
	return err
}

// ReadAttrs details: see vfs.DataProvider#ReadAttrs
func (dp *ftpDataProvider) ReadAttrs(path vfs.Path, dest interface{}) error {
	//this is ugly, because the current ftp implementation does not support the STAT request
	if info, ok := dest.(*vfs.ResourceInfo); ok {
		//do it by listing
		absPath := dp.Resolve(path)
		parentPath := vfs.Path(absPath).Parent().String()
		childName := path.Name()

		list, err := dp.conn.List(parentPath)
		if err != nil {
			return err
		}
		for _, entry := range list {
			if entry.Name == childName {
				info.Name = entry.Name
				info.ModTime = entry.Time.UnixNano() / 1e6
				switch entry.Type {
				case ftp.EntryTypeFile:
					info.Mode = 0
				case
					ftp.EntryTypeFolder:
					info.Mode = os.ModeDir
				case ftp.EntryTypeLink:
					info.Mode = os.ModeSymlink
				}
				info.Size = int64(entry.Size)
				return nil
			}
		}
		return &vfs.ResourceNotFoundError{Path: path}
	}
	return &vfs.UnsupportedAttributesError{Data: dest}
}

// WriteAttrs details: see vfs.DataProvider#WriteAttrs
func (dp *ftpDataProvider) WriteAttrs(path vfs.Path, src interface{}) error {
	return &vfs.UnsupportedOperationError{}
}

// ReadDir details: see vfs.DataProvider#ReadDir
func (dp *ftpDataProvider) ReadDir(path vfs.Path, options interface{}) (vfs.DirEntList, error) {
	entries, err := dp.conn.List(dp.Resolve(path))
	if err != nil {
		return nil, wrapErr(err)
	}

	tmp := make([]*ftp.Entry, len(entries))[0:0]
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		tmp = append(tmp, entry)
	}

	return vfs.NewDirEntList(int64(len(tmp)), func(idx int64, out *vfs.ResourceInfo) error {
		info := tmp[int(idx)]
		out.Name = info.Name
		switch info.Type {
		case ftp.EntryTypeFile:
			out.Mode = 0
		case
			ftp.EntryTypeFolder:
			out.Mode = os.ModeDir
		case ftp.EntryTypeLink:
			out.Mode = os.ModeSymlink
		}
		out.Size = int64(info.Size)
		out.ModTime = info.Time.UnixNano() / 1e6
		return nil
	}), nil
}

func (dp *ftpDataProvider) MkDirs(path vfs.Path) error {
	//optimistic creation first
	err := dp.conn.MakeDir(dp.Resolve(path))
	if protoErr, ok := err.(*textproto.Error); ok {
		if protoErr.Code == ftp.StatusFileUnavailable {
			//fallback to recursive behavior
			chain := ""
			for _, dir := range vfs.Path(dp.Resolve(path)).Names() {
				chain += "/" + dir
				if ok, _ := dp.exists(chain); !ok {
					err2 := dp.conn.MakeDir(chain)
					if err2 != nil {
						return wrapErr(err2)
					}
				}

			}

		}
	}
	return nil
}

//Exists returns true if the file exists, false if otherwise, error if something else
func (dp *ftpDataProvider) Exists(path vfs.Path) (bool, error) {
	return dp.exists(dp.Resolve(path))
}

func (dp *ftpDataProvider) exists(absPath string) (bool, error) {
	//TODO the ftp lib cannot stat a single entry
	list, err := dp.conn.NameList(vfs.Path(absPath).Parent().String())
	if err != nil {
		err = wrapErr(err)
		if _, is := err.(*vfs.ResourceNotFoundError); is {
			return false, nil
		}
		return false, err
	}
	expectName := vfs.Path(absPath).Name()

	for _, name := range list {
		if vfs.Path(name).Name() == expectName {
			return true, nil
		}
	}

	return false, nil
}

// Rename details: see vfs.DataProvider#Rename
func (dp *ftpDataProvider) Rename(oldPath vfs.Path, newPath vfs.Path) error {
	return dp.conn.Rename(dp.Resolve(oldPath), dp.Resolve(newPath))
}

// Close quits the ftp connection
func (dp *ftpDataProvider) Close() error {
	return dp.conn.Quit()
}

// The contract of the used ftp client is unusable and does not allow a byte sink.
// We have three choices, which are both bad:
//   1. buffer the entire file in memory or
//   2. buffer the entire file on-disk or
//   3. use some kind of piping
//
// The piping approach is complex and error prone and also requires a bunch of go routines. The provided pipe of the go
// SDK does work because it causes a deadlock. The best compromise, besides of forking and fixing the ftp API
// it to buffer the entire file on disk before uploading. However this breaks progress detection and needs a
// lot of disk space.
type diskBufferedWriter struct {
	file *os.File
	dp   *ftpDataProvider
	dst  string
}

func (w *diskBufferedWriter) ReadAt(b []byte, off int64) (n int, err error) {
	return w.file.ReadAt(b, off)
}

func (w *diskBufferedWriter) Read(p []byte) (n int, err error) {
	return w.file.Read(p)
}

func (w *diskBufferedWriter) WriteAt(b []byte, off int64) (n int, err error) {
	return w.file.WriteAt(b, off)
}

func (w *diskBufferedWriter) Write(p []byte) (n int, err error) {
	return w.file.Write(p)
}

func (w *diskBufferedWriter) Seek(offset int64, whence int) (int64, error) {
	return w.file.Seek(offset, whence)
}

func (w *diskBufferedWriter) Close() error {
	//seek to the beginning of the file
	_, err := w.file.Seek(0, io.SeekStart)
	if err != nil {
		_ = w.file.Close()
		return err
	}

	//try to actually transfer the data
	err = w.dp.conn.Stor(w.dst, w.file)
	if perr, ok := err.(*textproto.Error); ok {
		if perr.Code == ftp.StatusFileUnavailable {
			//retry by creating parent directory first
			err2 := w.dp.MkDirs(vfs.Path(w.dst).Parent())
			if err2 != nil {
				return err2
			}

			// oh, we created the parent successfully, retry the write
			err3 := w.dp.conn.Stor(w.dst, w.file)
			if err3 != nil {
				//intentionally return the first error
				return err
			}
		}
	}

	return w.file.Close()
}

// wrapErr inspects the ftp specific error and wraps it into something common as defined by the vfs itself
func wrapErr(err error) error {
	if perr, ok := err.(*textproto.Error); ok {
		switch perr.Code {
		case ftp.StatusFileUnavailable:
			return &vfs.ResourceNotFoundError{Cause: err}
		case ftp.StatusBadFileName:
			return &vfs.PermissionDeniedError{Cause: err}
		}
	}
	return err
}
