#include <sys/timerfd.h>
#include <sys/poll.h>
#include <sys/epoll.h>
#include <stdio.h>
#include <stdlib.h>
#include <strings.h>
#include <unistd.h>

// Example of timerfd_create with epoll.
int main(int ac, char *av[])
{
    struct epoll_event epollEvent;
    struct epoll_event newEvents;
    int timerfd;
    int epollfd;
    struct itimerspec timerValue;

    /* set timerfd */
    timerfd = timerfd_create(CLOCK_MONOTONIC, 0);
    if (timerfd < 0) {
        printf("failed to create timer fd\n");
        exit(1);
    }
    bzero(&timerValue, sizeof(timerValue));
    timerValue.it_value.tv_sec = 1;
    timerValue.it_value.tv_nsec = 0;
    timerValue.it_interval.tv_sec = 1;
    timerValue.it_interval.tv_nsec = 0;

    /* set events */
    epollfd = epoll_create1(0);
    epollEvent.events = EPOLLIN;
    epollEvent.data.fd = timerfd;
    epoll_ctl(epollfd, EPOLL_CTL_ADD, timerfd, &epollEvent);

    /* start timer */
    if (timerfd_settime(timerfd, 0, &timerValue, NULL) < 0) {
        printf("could not start timer\n");
        exit(1);
    }

    /* wait for events */
    while (1) {
        int numEvents = epoll_wait(epollfd, &newEvents, 1, 0);
        if (numEvents > 0) {
            int timersElapsed = 0;
            (void) read(epollEvent.data.fd, &timersElapsed, 8);
            printf("timers elapsed: %d\n", timersElapsed);
        }
    }

    exit(0);
}
