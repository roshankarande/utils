package toolbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// Client attempts to expose guest.OperationsManager as idiomatic Go interfaces
type Client struct {
	ProcessManager *guest.ProcessManager
	FileManager    *guest.FileManager
	Authentication types.BaseGuestAuthentication
	GuestFamily    types.VirtualMachineGuestOsFamily
}

func (c *Client) rm(ctx context.Context, path string) {
	err := c.FileManager.DeleteFile(ctx, c.Authentication, path)
	if err != nil {
		log.Printf("rm %q: %s", path, err)
	}
}

func (c *Client) mktemp(ctx context.Context) (string, error) {
	return c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "govmomi-", "", "")
}

type exitError struct {
	error
	exitCode int
}

func (e *exitError) ExitCode() int {
	return e.exitCode
}

// Run implements exec.Cmd.Run over vmx guest RPC against standard vmware-tools or toolbox.
func (c *Client) Run(ctx context.Context, cmd *exec.Cmd, data chan string) error {
	defer close(data)

	output := []struct {
		io.Writer
		fd   string
		path string
	}{
		{cmd.Stdout, "", ""},
		//{cmd.Stderr, "", ""},
	}

	for i, out := range output {
		if out.Writer == nil {
			continue
		}

		dst, err := c.mktemp(ctx)
		if err != nil {
			return err
		}

		defer c.rm(ctx, dst)

		//cmd.Args = append(cmd.Args, out.fd+">", dst)
		cmd.Args = append(cmd.Args, out.fd+"| Out-File ", dst, " -encoding ASCII")
		output[i].path = dst
	}

	path := cmd.Path
	args := cmd.Args

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		// Using 'cmd.exe /c' is required on Windows for i/o redirection
		//	path = "c:\\Windows\\System32\\cmd.exe"
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		//args = append([]string{"/c", cmd.Path}, args...)
		args = append([]string{cmd.Path}, args...)
	default:
		if !strings.ContainsAny(cmd.Path, "/") {
			// vmware-tools requires an absolute ProgramPath
			// Default to 'bash -c' as a convenience
			path = "/bin/bash"
			arg := "'" + strings.Join(append([]string{cmd.Path}, args...), " ") + "'"
			args = []string{"-c", arg}
		}
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		EnvVariables:     cmd.Env,
		WorkingDirectory: cmd.Dir,
	}

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	var l = []int64{0, 0} // l[0] - stdoutput len... l[1] - stderr len

	for {

		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		if err != nil {
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best....

			for index, out := range output {
				var buf = new(strings.Builder)
				if out.Writer == nil {
					continue
				}

				f, n, err := c.Download(ctx, out.path)
				if err != nil {
					return err
				}

				io.Copy(buf, f)
				if index == 0 {
					//fmt.Print(buf.String()[l[index]:n])
					data <- buf.String()[l[index]:n]
					l[index] = n
				}
			}
			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)
	for index, out := range output {
		if out.Writer == nil {
			continue
		}

		f, n, err := c.Download(ctx, out.path)
		if err != nil {
			return err
		}

		io.Copy(buf, f)
		if index == 0 {
			data <- buf.String()[l[0]:n]
			//fmt.Print(buf.String()[l[0]:n])
		}
	}

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", cmd.Path, rc), rc}
	}

	return nil
}

// Run implements exec.Cmd.Run over vmx guest RPC against standard vmware-tools or toolbox.
func (c *Client) RunCommands(ctx context.Context, data chan string, commands []string) error {
	defer close(data)

	output := []struct {
		io.Writer
		fd   string
		path string
	}{
		{os.Stdout, "", ""},
		//	{os.Stderr, "", ""},
	}

	for i, out := range output {
		if out.Writer == nil {
			continue
		}

		dst, err := c.mktemp(ctx)
		if err != nil {
			return err
		}

		defer c.rm(ctx, dst)

		//cmd.Args = append(cmd.Args, out.fd+">", dst)
		//cmd.Args = append(cmd.Args, out.fd+"| Out-File ", dst," -encoding ASCII")
		output[i].path = dst
	}

	//path := cmd.Path
	//args := cmd.Args
	var path string
	var args []string

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		// Using 'cmd.exe /c' is required on Windows for i/o redirection
		//	path = "c:\\Windows\\System32\\cmd.exe"
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		//args = append([]string{"/c", cmd.Path}, args...)
		//args = append([]string{ cmd.Path }, args...)
		args = []string{"-Command", fmt.Sprintf(`"& { %s }"`, strings.Join(commands, ";")), "| Out-File", output[0].path, "-encoding ASCII"}

	default:
		//if !strings.ContainsAny(cmd.Path, "/") {
		//	// vmware-tools requires an absolute ProgramPath
		//	// Default to 'bash -c' as a convenience
		//	path = "/bin/bash"
		//	arg := "'" + strings.Join(append([]string{cmd.Path}, args...), " ") + "'"
		//	args = []string{"-c", arg}
		//}
		fmt.Errorf("not a windows machine")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	fmt.Println(spec.ProgramPath, spec.Arguments)

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	var l = []int64{0, 0} // l[0] - stdoutput len... l[1] - stderr len

	for {

		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		if err != nil {
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best....

			for index, out := range output {
				var buf = new(strings.Builder)
				if out.Writer == nil {
					continue
				}

				f, n, err := c.Download(ctx, out.path)
				if err != nil {
					return err
				}

				io.Copy(buf, f)
				//fmt.Print(buf.String()[l[index]:n])

				if index == 0 {
					data <- buf.String()[l[index]:n]
					l[index] = n
				}
			}

			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)
	for index, out := range output {
		if out.Writer == nil {
			continue
		}

		f, n, err := c.Download(ctx, out.path)
		if err != nil {
			return err
		}

		io.Copy(buf, f)
		if index == 0 {
			data <- buf.String()[l[0]:n]
			//fmt.Print(buf.String()[l[0]:n])
		}
	}

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	return nil

}

// Run implements RunCommand over vmx guest RPC against standard vmware-tools or toolbox.
func (c *Client) RunCommand(ctx context.Context, data chan string, command string) error {
	output := []struct {
		io.Writer
		fd   string
		path string
	}{
		{os.Stdout, "", ""},
		//	{os.Stderr, "", ""},
	}

	for i, out := range output {
		if out.Writer == nil {
			continue
		}

		dst, err := c.mktemp(ctx)
		if err != nil {
			return err
		}

		defer c.rm(ctx, dst)

		//cmd.Args = append(cmd.Args, out.fd+">", dst)
		//cmd.Args = append(cmd.Args, out.fd+"| Out-File ", dst," -encoding ASCII")
		output[i].path = dst
	}

	var path string
	var args []string

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		args = []string{"-Command", fmt.Sprintf(`"& { %s }"`, command), "| Out-File", output[0].path, "-encoding ASCII"}
	default:
		fmt.Errorf("not a windows machine")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	var l = []int64{0, 0} // l[0] - stdoutput len... l[1] - stderr len

	for {

		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		if err != nil {
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10) // see what fits best....

			for index, out := range output {
				var buf = new(strings.Builder)
				if out.Writer == nil {
					continue
				}

				f, n, err := c.Download(ctx, out.path)
				if err != nil {
					return err
				}

				io.Copy(buf, f)
				//fmt.Print(buf.String()[l[index]:n])
				if index == 0 {
					data <- buf.String()[l[index]:n]
					l[index] = n
				}

			}

			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)
	for index, out := range output {
		if out.Writer == nil {
			continue
		}

		f, n, err := c.Download(ctx, out.path)
		if err != nil {
			return err
		}

		io.Copy(buf, f)
		//io.Copy(stdout,f)

		if index == 0 {
			data <- buf.String()[l[0]:n]
			//fmt.Print(buf.String()[l[0]:n])
		}
	}

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	return nil
}

// full run.... add functionality to RunCommands and remove this function
func (c *Client) RunSimpleCommands(ctx context.Context, commands []string) error {

	output := []struct {
		io.Writer
		fd   string
		path string
	}{
		{os.Stdout, "", ""},
		//	{os.Stderr, "", ""},
	}

	for i, out := range output {
		if out.Writer == nil {
			continue
		}

		dst, err := c.mktemp(ctx)
		if err != nil {
			return err
		}

		defer c.rm(ctx, dst)

		output[i].path = dst
	}

	var path string
	var args []string

	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		args = []string{"-Command", fmt.Sprintf(`"& { %s }"`, strings.Join(commands, ";")), "| Out-File", output[0].path, "-encoding ASCII"}
	default:
		fmt.Errorf("not a windows machine")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	for {
		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		if err != nil {
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 10)
			continue
		}

		rc = int(p.ExitCode)
		break
	}

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	return nil
}

// RunScript implements RunScript over vmx guest RPC against standard vmware-tools or toolbox.
func (c *Client) RunScript(ctx context.Context, data chan string, script string) error {
	defer close(data)

	execfile, err := c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "govmomi-", ".ps1", "")
	if err != nil {
		return err
	}
	defer c.rm(ctx, execfile)

	fSrcScript := strings.NewReader(script)

	if err != nil {
		return err
	}
	p := soap.DefaultUpload

	err = c.Upload(ctx, fSrcScript, execfile, p, &types.GuestFileAttributes{}, true)

	if err != nil {
		fmt.Println(err)
		return err
	}

	outFile, err := c.mktemp(ctx)
	if err != nil {
		return err
	}
	defer c.rm(ctx, outFile)

	var path string
	var args []string
	switch c.GuestFamily {
	case types.VirtualMachineGuestOsFamilyWindowsGuest:
		path = "C:\\WINDOWS\\system32\\WindowsPowerShell\\v1.0\\powershell.exe"
		args = []string{execfile, "| Out-File", outFile, "-encoding ASCII"}
	default:
		//if !strings.ContainsAny(cmd.Path, "/") {
		//	// vmware-tools requires an absolute ProgramPath
		//	// Default to 'bash -c' as a convenience
		//	path = "/bin/bash"
		//	arg := "'" + strings.Join(append([]string{cmd.Path}, args...), " ") + "'"
		//	args = []string{"-c", arg}
		//}
		return fmt.Errorf("not a windows system")
	}

	spec := types.GuestProgramSpec{
		ProgramPath:      path,
		Arguments:        strings.Join(args, " "),
		WorkingDirectory: "",
		EnvVariables:     nil,
	}

	//fmt.Println(spec.ProgramPath,spec.Arguments)

	pid, err := c.ProcessManager.StartProgram(ctx, c.Authentication, &spec)
	if err != nil {
		return err
	}

	rc := 0
	var l int64 = 0

	for {
		procs, err := c.ProcessManager.ListProcesses(ctx, c.Authentication, []int64{pid})
		if err != nil {
			return err
		}

		p := procs[0]

		if p.EndTime == nil {
			<-time.After(time.Second * 2) // see what fits best....

			var buf = new(strings.Builder)

			f, n, err := c.Download(ctx, outFile)
			if err != nil {
				return err
			}

			io.Copy(buf, f)
			data <- buf.String()[l:n]
			l = n

			continue
		}

		rc = int(p.ExitCode)
		break
	}

	var buf = new(strings.Builder)
	f, n, err := c.Download(ctx, outFile)
	if err != nil {
		return err
	}

	io.Copy(buf, f)
	data <- buf.String()[l:n]

	if rc != 0 {
		return &exitError{fmt.Errorf("%s: exit %d", path, rc), rc}
	}

	//fmt.Println(buf.String())

	return nil
}

// archiveReader wraps an io.ReadCloser to support streaming download
// of a guest directory, stops reading once it sees the stream trailer.
// This is only useful when guest tools is the Go toolbox.
// The trailer is required since TransferFromGuest requires a Content-Length,
// which toolbox doesn't know ahead of time as the gzip'd tarball never touches the disk.
// We opted to wrap this here for now rather than guest.FileManager so
// DownloadFile can be also be used as-is to handle this use case.
type archiveReader struct {
	io.ReadCloser
}

var (
	gzipHeader    = []byte{0x1f, 0x8b, 0x08} // rfc1952 {ID1, ID2, CM}
	gzipHeaderLen = len(gzipHeader)
)

func (r *archiveReader) Read(buf []byte) (int, error) {
	nr, err := r.ReadCloser.Read(buf)

	// Stop reading if the last N bytes are the gzipTrailer
	if nr >= gzipHeaderLen {
		if bytes.Equal(buf[nr-gzipHeaderLen:nr], gzipHeader) {
			nr -= gzipHeaderLen
			err = io.EOF
		}
	}

	return nr, err
}

func isDir(src string) bool {
	u, err := url.Parse(src)
	if err != nil {
		return false
	}

	return strings.HasSuffix(u.Path, "/")
}

// Download initiates a file transfer from the guest
func (c *Client) Download(ctx context.Context, src string) (io.ReadCloser, int64, error) {
	vc := c.ProcessManager.Client()

	info, err := c.FileManager.InitiateFileTransferFromGuest(ctx, c.Authentication, src)
	if err != nil {
		return nil, 0, err
	}

	u, err := c.FileManager.TransferURL(ctx, info.Url)
	if err != nil {
		return nil, 0, err
	}

	p := soap.DefaultDownload

	f, n, err := vc.Download(ctx, u, &p)
	if err != nil {
		return nil, n, err
	}

	if strings.HasPrefix(src, "/archive:/") || isDir(src) {
		f = &archiveReader{ReadCloser: f} // look for the gzip trailer
	}

	return f, n, nil
}

// Upload transfers a file to the guest
func (c *Client) Upload(ctx context.Context, src io.Reader, dst string, p soap.Upload, attr types.BaseGuestFileAttributes, force bool) error {
	vc := c.ProcessManager.Client()

	var err error

	if p.ContentLength == 0 { // Content-Length is required
		switch r := src.(type) {
		case *bytes.Buffer:
			p.ContentLength = int64(r.Len())
		case *bytes.Reader:
			p.ContentLength = int64(r.Len())
		case *strings.Reader:
			p.ContentLength = int64(r.Len())
		case *os.File:
			info, serr := r.Stat()
			if serr != nil {
				return serr
			}

			p.ContentLength = info.Size()
		}

		if p.ContentLength == 0 { // os.File for example could be a device (stdin)
			buf := new(bytes.Buffer)

			p.ContentLength, err = io.Copy(buf, src)
			if err != nil {
				return err
			}

			src = buf
		}
	}

	url, err := c.FileManager.InitiateFileTransferToGuest(ctx, c.Authentication, dst, attr, p.ContentLength, force)
	if err != nil {
		return err
	}

	u, err := c.FileManager.TransferURL(ctx, url)
	if err != nil {
		return err
	}

	return vc.Client.Upload(ctx, src, u, &p)
}

// customized Function
func (c *Client) UploadScript(ctx context.Context, dst string, f io.Reader) error {
	return c.UploadFile(ctx,dst,f,false)
}

// customized Function
func (c *Client) UploadFile(ctx context.Context, dst string, f io.Reader, isDir bool) error {

	vcFile, err := c.FileManager.CreateTemporaryFile(ctx, c.Authentication, "", "", "")

	if err != nil {
		return err
	}

	defer c.FileManager.DeleteFile(ctx, c.Authentication, vcFile)

	p := soap.DefaultUpload
	err = c.Upload(ctx, f, vcFile, p, &types.GuestFileAttributes{}, true)
	if err != nil {
		return err
	}

	if isDir {
		cmd := fmt.Sprintf("tar -xzvf %s -C %s", vcFile, dst)
		c.RunSimpleCommands(ctx, []string{fmt.Sprintf("mkdir %s -Force", dst), cmd})
	} else {
		err = c.FileManager.MoveFile(ctx, c.Authentication, vcFile, dst, true)
		if err != nil {
			return err
		}
	}

	return nil
}
