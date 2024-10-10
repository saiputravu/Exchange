#include "session.h"
#include <boost/asio.hpp>
#include <cstddef>
#include <iostream>
#include <iterator>
#include <memory>
#include <string>

void Session::wait_for_request() {
  // We're passing a callback to async_read_until, which
  // can be executed at anytime (when data arrives).
  // We need a valid reference to `this`, which can go
  // out of scope when the subsequent exeuction happens.
  std::cout << "New session" << std::endl;
  std::shared_ptr<Session> self(shared_from_this());

  // Read the data off the socket.
  boost::asio::async_read_until(
      socket, buffer, "\0",
      [this, self](boost::system::error_code ec, std::size_t length) {
        if (!ec) {
          // Do something with the data buffer
          std::string stringified{std::istreambuf_iterator<char>(&buffer),
                                  std::istreambuf_iterator<char>()};
          std::cout << "Received: " << stringified << std::endl;
          // call back to wait for next request
          wait_for_request();
        } else {
          // Error reading off buffer
          std::cout << "error " << ec << std::endl;
        }
      });
}
