#include "communication/server.h"
#include <boost/asio/io_context.hpp>
#include <iostream>

int main(int argc, char *argv[]) {
  std::cout << "Running exchange server" << std::endl;
  boost::asio::io_context io_context;
  ExchangeServer server(io_context, 25000);
  io_context.run();
  return 0;
}
