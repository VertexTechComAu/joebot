# joebot

Golang Command &amp; Control Server For Managing And Remote Accessing Machines Via Web Interface

## :raising_hand: Motivations

The big motivation is as follows.

- :tired_face: Hard to access into CI builders for troubleshooting issues which are not reproducible locally
- :tired_face: Tedious to forward particular port on CI builders for debugging
- :pray: Provide an easy way for accessing builder machines

## :cd: Installation

Directly download the binary from <https://github.com/harmonicinc-com/joebot/releases>

## :star: Features

- Secure Remote Access: All connections are secured with TLS encryption.
- IP Whitelisting: Access to the web portal and all tunneled services is restricted to whitelisted IP addresses.
- Web Terminal: Instant terminal access in your browser.
- Web File Browser: Manage files on remote machines with a web-based interface.
- Web VNC: Remote desktop access (Defaults to port 5901).
- Dynamic Port Tunnelling: Forward any port from a client to the server.
- User Management: Add and remove users for the web portal.

## :lock: Security

Joebot uses TLS to secure communication between the server and clients. Both the server and clients must be configured with the appropriate certificates. Additionally, the web portal features a user authentication system with IP whitelisting to ensure that only authorized users from trusted IP addresses can access the system.

## Usage (Control Server)

```bash
joebot server --port=<Server_Port> --web-portal-port=<Server_Web_Portal_Port> --ca-cert=<CA_Cert_Path> --server-cert=<Server_Cert_Path> --server-key=<Server_Key_Path> --db-path=<DB_Path>
```

## Usage (Client)

```bash
joebot client --port=<Server_Port> --tag=customized-client-id --ca-cert=<CA_Cert_Path> --client-cert=<Client_Cert_Path> --client-key=<Client_Key_Path> <Server_IP>
```

### User Management

You can manage users for the web portal using the `user` command:

Add a new user:

```bash
joebot user --db-path=<DB_Path> add <username> <password>
```

Delete a user:

```bash
joebot user --db-path=<DB_Path> del <username>
```

### Web Interface

![Screenshot](https://raw.githubusercontent.com/harmonicinc-com/joebot/master/screenshot.PNG)

### Terminal Via Web Browser

![Screenshot](https://raw.githubusercontent.com/harmonicinc-com/joebot/master/screenshot-terminal.PNG)

### VNC Via Web Browser

![Screenshot](https://raw.githubusercontent.com/harmonicinc-com/joebot/master/screenshot-vnc.PNG)

### File Manager Via Web Browser

![Screenshot](https://raw.githubusercontent.com/harmonicinc-com/joebot/master/screenshot-filebrowser.gif)
