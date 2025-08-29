# qreph
Go cli that creates a one time note which is served it over HTTP, and shows a QR code for mobile scanning. The note self destructs after the first view (ephemeral) hence qr(eph).

# How to run
'''sh
go build
'''

'''sh 
./qreph "your content"
'''
or pipe 
'''sh
echo "your content" | ./qreph
'''
