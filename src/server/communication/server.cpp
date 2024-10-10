#include "server.h"
#include "session.h"
#include <boost/asio/io_context.hpp>
#include <boost/asio/ip/tcp.hpp>
#include <cstdint>
#include <iostream>
#include <memory>
#include <netinet/in.h>
#include <sys/socket.h>

using namespace boost::asio::ip;

ExchangeServer::ExchangeServer(boost::asio::io_context &context, uint16_t port)
    : acceptor(context, tcp::endpoint(tcp::v4(), port)), port(port) {
  // Start waiting for the first client to connect to the server.
  start_accept();
}

void ExchangeServer::start_accept() {
  acceptor.async_accept(
      [this](boost::system::error_code ec, tcp::socket socket) {
        if (!ec) {
          // No error on connection accept. Create a new session.
          // Run the session and give up ownership on this socket.
          std::make_shared<Session>(std::move(socket))->run();
        } else {
          // Error on connection accept.
          std::cout << "error " << ec << std::endl;
        }

        // We wait for the next client, as we want multiple clients to connect,
        // by re-calling this function.
        start_accept();
      });
}
