# Timer-event-handling-by-epoll


This experimental code is to test the epoll based handling of different signals including the timer events and packet arrival events. 
The software initializes the epoll instance and register the fd of a timer. Whenever the timer expires, 
the epoll_wait() returns the timer fd. Then the software transmit a icmp request to a machine in the Lab.

Whenever the icmp response arrives to a icmp listening socket, epoll instance signals the arrival of the icmp response giving the socket's fd
as a return value of epoll_wait()

