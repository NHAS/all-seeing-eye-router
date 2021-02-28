## Network notifications
  
Do you run a router that is actually a general purpose computer?  
Do you want to know of devices comings and goings?  

Then this project is for you! 

Functions kind of like `ip neigh` by querying the kernel routing tables (rtnetlink https://man7.org/linux/man-pages/man7/rtnetlink.7.html) and essentially returning changes in state 'liveness'. 

E.g 

If a device suddenly goes from "Failed" resolution to "Reachable" then that notification is passed to the client via golang RPC. 


## Features

- Gives events of devices becoming live on the network
- Has a client to display notifications via Dbus (dunstify)


## Todo

- Client side 'labelling' of devices to allow for friendly names to be applied
- Use unix sockets to allow `ls` like network query