File sharing server with upload via HTTP, written in Go.

Motivation
==========

You may find this software useful if you own a web server and occasionally:

* want to store/share a file online, but can't/don't want to login to your
  email/file hosting account or ssh to your server (e.g. using a public PC);
* don't want to manually cleanup temporary shared files.

Features
========

* No-frills web interface.
* Password protected file upload (warning: no encryption; modify the code to
  use ListenAndServeTLS or see example lighttpd configuration below for a
  secure configuration).
* Access from a list (public files) or a direct link (private files).
* Limit access by expiration date or number of downloads (optionally deleting
  the file from the server afterwards).

Usage
=====

	go-share [-d DIRECTORY] [-p PORT]

Runs on the port PORT (default: 80), stores files in the directory DIRECTORY
(defaults to current directory).

Example
=======

	$ mkdir shares
	$ go-share -d shares -p 9321
	Please enter a password for file upload:
	Please repeat the password:
	Starting go-share on port 9321.

Example lighttpd configuration
------------------------------

To make go-share appear on http://example.com/shared/ add the following to your
lighttpd.conf (assuming you used the port number as in the example above):

	$HTTP["url"] =~ "^/shared/" {
	     proxy.server  = ( "" => ( ("host" => "127.0.0.1", "port" => "9321") ) )
	}

If you have a SSL certificate make sure to protect your password by using
https. Example lighttpd configuration:

	$SERVER["socket"] == ":443" {
	    ssl.engine                  = "enable" 
	    ssl.pemfile                 = "/etc/lighttpd/ssl/example.com.pem" 
	}

Hint
=====

To delete a file named X, upload anything with the file name X and expiration
date set to something in the past.
