module nfsclient

go 1.22.6

replace github.com/vmware/go-nfs-client => /home/nutanix/sftp-go/go-nfs-client/

require (
	github.com/rasky/go-xdr v0.0.0-20170124162913-1a41d1a06c93
	github.com/vmware/go-nfs-client v0.0.0-20190605212624-d43b92724c1b
)
