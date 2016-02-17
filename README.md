# check_puppetserver
Small Nagios to test Puppet 4 servers

```
$ ./check_puppetserver --help
Usage of ./check_puppetserver:
  -H string
        Hostname to query, defaults to localhost (default "localhost")
  -c float
        CRITICAL threshold in seconds, defaults to 15 seconds (default 15)
  -cert string
        A PEM eoncoded client certificate file, defaults to /etc/puppetlabs/puppet/ssl/certs/$(hostname -f).pem (default "/etc/puppetlabs/puppet/ssl/certs/$(hostname -f).pem")
  -debug
        log debug output, defaults to false
  -e string
        Puppet environment to ask for, defaults to production (default "production")
  -key string
        A PEM encoded private key file for the client certificate, defaults to /etc/puppetlabs/puppet/ssl/private_keys/$(hostname -f).pem (default "/etc/puppetlabs/puppet/ssl/private_keys/$(hostname -f).pem")
  -p int
        Port to send the query to, defaults to 8140 (default 8140)
  -w float
        WARNING threshold in seconds, defaults to 5 seconds (default 5)
```

```
$ ./check_puppetserver -H puppet -key key.pem -cert cert.pem 
OK: Puppet Server (Version: 4.3.1) looks good, checked for Puppet environment production in 0.06821s|time=0.06821s
```
```
$ ./check_puppetserver -H puppet -key key.pem -cert cert.pem -debug
DEBUG Certificate file: cert.pem found.
DEBUG Certificate file: key.pem found.
DEBUG Trying to load cert file: cert.pem and key file: key.pem
DEBUG Sending query https://puppet:8140/puppet/v3/status/whatever?environment=production took 0.07033s
DEBUG Response is: {"is_alive":true,"version":"4.3.1"}
OK: Puppet Server (Version: 4.3.1) looks good, checked for Puppet environment production in 0.07033s|time=0.07033s
```
```
$ ./check_puppetserver -H puppet -key key.pem -cert cert.pem -w 0.003
WARNING: Response time 3.65815s >= 0.00s - Puppet Server (Version: 4.3.1) looks good, checked for Puppet environment production in 3.65815s|time=3.65815s
$ echo $?
1
```
