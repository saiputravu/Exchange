#pragma once

/*
 * Long-standing exchange server.
 *
 * This setups a socket listener on a given address and port.
 *
 * When a user connects to the server, the connection is handled,
 * and passed over to a Session object to handle communication.
 */
#include <boost/asio/io_context.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <cstdint>
#include <string>

using namespace boost::asio::ip;

class ExchangeServer {
public:
  ExchangeServer(boost::asio::io_context &context, uint16_t port);

private:
  void start_accept();

  uint16_t port;
  boost::asio::io_context io_context;
  tcp::acceptor acceptor;
};
