package vfsftp

import (
	"bytes"
	"github.com/jlaffaye/ftp"
	"github.com/worldiety/vfs"
	"io"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
)

type ftpDataProvider struct {
	conn   *ftp.ServerConn
	Prefix string
}

// Connect opens the ftp and performs a login using information from the url.
// This FTP implementation is NOT thread safe, because it only ever uses a single connection which is
// stateful.
func Connect(url *url.URL) (vfs.DataProvider, error) {
	adr := url.Host
	login := url.User.Username()
	password, _ := url.User.Password()
	pathPrefix := url.Path

	conn, err := ftp.Connect(adr)
	if err != nil {
		return nil, err
	}
	err = conn.Login(login, password)
	if err != nil {
		return nil, err
	}
	return &ftpDataProvider{conn, pathPrefix}, nil
}

// Resolve creates a platform specific filename from the given invariant path by adding the Prefix and using
// the platform specific name separator. If AllowRelativePaths is false (default), .. will be silently ignored.
func (dp *ftpDataProvider) Resolve(path vfs.Path) string {
	if len(dp.Prefix) == 0 {
		return path.String()
	}
	// security feature: we normalize our path, before adding the prefix to avoid breaking out of our root
	path = path.Normalize()
	return vfs.Path(filepath.Join(dp.Prefix, filepath.Join(path.Names()...))).String()
}

// Read details: see vfs.DataProvider#Read
func (dp *ftpDataProvider) Read(path vfs.Path) (io.ReadCloser, error) {
	res, err := dp.conn.Retr(dp.Resolve(path))
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Write details: see vfs.DataProvider#Write
func (dp *ftpDataProvider) Write(path vfs.Path) (io.WriteCloser, error) {
	return &bufferedWriter{&bytes.Buffer{}, dp, dp.Resolve(path)}, nil
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
	}
	return &vfs.UnsupportedAttributesError{Data: dest}
}

// WriteAttrs details: see vfs.DataProvider#WriteAttrs
func (dp *ftpDataProvider) WriteAttrs(path vfs.Path, src interface{}) error {
	return &vfs.UnsupportedOperationError{}
}

// ReadDir details: see vfs.DataProvider#ReadDir
func (dp *ftpDataProvider) ReadDir(path vfs.Path) (vfs.DirEntList, error) {
	entries, err := dp.conn.List(path.String())
	if err != nil {
		return nil, err
	}

	tmp := make([]*ftp.Entry, len(entries))[0:0]
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		tmp = append(tmp, entry)
	}

	return &fileInfoDirEntList{tmp}, nil
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
						return err2
					}
				}

			}

		}
	}
	return nil
}

func (dp *ftpDataProvider) exists(absPath string) (bool, error) {
	_, err := dp.conn.FileSize(absPath)
	if protoErr, ok := err.(*textproto.Error); ok {
		if protoErr.Code == ftp.StatusFileUnavailable {
			return false, nil
		}
	}
	if err != nil {
		return true, nil
	}
	return false, err
}

// Rename details: see vfs.DataProvider#Rename
func (dp *ftpDataProvider) Rename(oldPath vfs.Path, newPath vfs.Path) error {
	return dp.conn.Rename(dp.Resolve(oldPath), dp.Resolve(newPath))
}

// Close quits the ftp connection
func (dp *ftpDataProvider) Close() error {
	return dp.conn.Quit()
}

type fileInfoDirEntList struct {
	list []*ftp.Entry
}

func (l *fileInfoDirEntList) ForEach(each func(scanner vfs.Scanner) error) error {
	scanner := &fileScanner{}
	for _, info := range l.list {
		scanner.info = info
		err := each(scanner)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *fileInfoDirEntList) Size() int64 {
	return int64(len(l.list))
}

// Close does nothing
func (l *fileInfoDirEntList) Close() error {
	return nil
}

//
type fileScanner struct {
	info *ftp.Entry
}

func (f *fileScanner) Scan(dest interface{}) error {
	if out, ok := dest.(*vfs.ResourceInfo); ok {
		out.Name = f.info.Name
		switch f.info.Type {
		case ftp.EntryTypeFile:
			out.Mode = 0
		case
			ftp.EntryTypeFolder:
			out.Mode = os.ModeDir
		case ftp.EntryTypeLink:
			out.Mode = os.ModeSymlink
		}
		out.Size = int64(f.info.Size)
		out.ModTime = f.info.Time.UnixNano() / 1e6
		return nil
	}
	return &vfs.UnsupportedAttributesError{Data: dest}
}

// TODO buffering in memory is a bad idea but piping is also ugly because of the extra go routine and channel logic
type bufferedWriter struct {
	buf  *bytes.Buffer
	dp   *ftpDataProvider
	path string
}

func (b *bufferedWriter) Write(p []byte) (n int, err error) {
	return b.buf.Write(p)
}

func (b *bufferedWriter) Close() error {
	err := b.dp.conn.Stor(b.path, bytes.NewReader(b.buf.Bytes()))
	if perr, ok := err.(*textproto.Error); ok {
		if perr.Code == ftp.StatusFileUnavailable {
			//retry by creating parent directory first
			err2 := b.dp.MkDirs(vfs.Path(b.path).Parent().TrimPrefix(vfs.Path(b.dp.Prefix)))
			if err2 != nil {
				return err2
			}

			// oh, we created the parent successfully, retry the write
			err3 := b.dp.conn.Stor(b.path, bytes.NewReader(b.buf.Bytes()))
			if err3 != nil {
				//intentionally return the first error
				return err
			}
		}
	}
	return nil
}
