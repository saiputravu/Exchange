add_subdirectory(server)

file (GLOB exchange_server_SRC
 "server/*.h"
 "server/*.cpp"
 "server/communication/*.h"
 "server/communication/*.cpp"
)

add_executable(exchange_server ${exchange_server_SRC})
