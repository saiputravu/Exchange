#pragma once

#include <boost/asio/ip/tcp.hpp>
#include <boost/asio/streambuf.hpp>
#include <memory>

using namespace boost::asio::ip;

class Session : public std::enable_shared_from_this<Session> {
public:
  Session(tcp::socket socket) : socket(std::move(socket)) {}

  void run() { wait_for_request(); }

private:
  void wait_for_request();

  tcp::socket socket;
  boost::asio::streambuf buffer;
};
