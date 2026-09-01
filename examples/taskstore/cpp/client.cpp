/**
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */


// The C++ client for the taskstore server.
//
//   g++ -O2 -std=c++17 -I../../../lib/cpp -o client client.cpp
//   ./client list
//   ./client create "Buy milk" normal errand,home 1755820800
//   ./client read 1001
//   ./client update 1001 "Buy oat milk" true high errand
//   ./client delete 1001
//
// Note the -I: the client needs serify.hpp on the include path even though it
// never calls a line of it. The schema binding in C++ is a set of macros, and
// macros have to be expanded, so model.hpp includes the header and everything
// downstream of it inherits that. Go and Python are the only two of the nine
// whose codec can be free of the harness at compile time; the README's
// "Where serify appears" section has the full split.

#include "frame.hpp"
#include "message.hpp"
#include "model.hpp"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <iostream>
#include <string>
#include <vector>

#include <arpa/inet.h>
#include <netdb.h>
#include <sys/socket.h>
#include <unistd.h>

namespace {

const char* USAGE =
    "usage: client [--addr host:port] <command> [args]\n"
    "\n"
    "  list\n"
    "  create <title> [priority] [tag,tag] [due-unix]\n"
    "  read   <id>\n"
    "  update <id> <title> <done> [priority] [tag,tag] [due-unix]\n"
    "  delete <id>\n"
    "\n"
    "priority is low, normal or high (default normal).";

std::string arg_at(const std::vector<std::string>& args, size_t i, const std::string& fallback) {
    return (i < args.size() && !args[i].empty()) ? args[i] : fallback;
}

uint64_t parse_id(const std::string& raw) {
    if (raw.empty() || raw.find_first_not_of("0123456789") != std::string::npos)
        throw std::runtime_error("bad id \"" + raw + "\"");
    return std::strtoull(raw.c_str(), nullptr, 10);
}

/// Rejects a bad priority here, before anything is encoded. This is the only
/// place in the project where one can exist: an enum has no wire representation
/// outside its declared variants, so by the time a request is bytes the value is
/// already known to be good — which is why the server does not check it again.
std::string parse_priority(const std::string& s) {
    for (const auto& p : taskstore::PRIORITIES)
        if (p == s) return s;
    throw std::runtime_error("\"" + s + "\" is not a priority (low, normal, high)");
}

std::vector<std::string> parse_tags(const std::string& s) {
    std::vector<std::string> out;
    if (s.empty()) return out;
    size_t start = 0;
    while (true) {
        const size_t comma = s.find(',', start);
        out.push_back(s.substr(start, comma == std::string::npos ? comma : comma - start));
        if (comma == std::string::npos) break;
        start = comma + 1;
    }
    return out;
}

std::optional<int64_t> parse_due(const std::string& s) {
    if (s.empty()) return std::nullopt;
    try {
        return static_cast<int64_t>(std::stoll(s));
    } catch (...) {
        return std::nullopt;
    }
}

taskstore::Op parse_op(const std::vector<std::string>& args) {
    if (args.empty()) throw std::runtime_error("no command given");
    const std::string& command = args[0];
    taskstore::Op op;

    if (command == "list") {
        op.emplace<0>();
    } else if (command == "create") {
        if (args.size() < 2) throw std::runtime_error("create needs a title");
        taskstore::Draft d{};
        d.title    = args[1];
        d.priority = parse_priority(arg_at(args, 2, "normal"));
        d.tags     = parse_tags(arg_at(args, 3, ""));
        d.due_at   = parse_due(arg_at(args, 4, ""));
        op.emplace<1>(std::move(d));
    } else if (command == "read") {
        if (args.size() < 2) throw std::runtime_error("read needs an id");
        op.emplace<2>(parse_id(args[1]));
    } else if (command == "update") {
        if (args.size() < 4) throw std::runtime_error("update needs an id, a title and done");
        if (args[3] != "true" && args[3] != "false")
            throw std::runtime_error("bad done \"" + args[3] + "\", want true or false");
        taskstore::Task t{};
        t.id       = parse_id(args[1]);
        t.title    = args[2];
        t.done     = args[3] == "true";
        t.priority = parse_priority(arg_at(args, 4, "normal"));
        t.tags     = parse_tags(arg_at(args, 5, ""));
        t.due_at   = parse_due(arg_at(args, 6, ""));
        op.emplace<3>(std::move(t));
    } else if (command == "delete") {
        if (args.size() < 2) throw std::runtime_error("delete needs an id");
        op.emplace<4>(parse_id(args[1]));
    } else {
        throw std::runtime_error("unknown command \"" + command + "\"");
    }
    return op;
}

std::string render_task(const taskstore::Task& t) {
    std::string line = std::string("[") + (t.done ? "x" : " ") + "] " + std::to_string(t.id) + "  ";
    std::string title = t.title;
    // Padded by bytes, because std::string counts bytes. Go and Python count
    // runes, so a non-ASCII title lines up differently there. That is display
    // only — nothing in this padding reaches the wire, where the length prefix
    // has always been a byte count in every language.
    if (title.size() < 30) title.append(30 - title.size(), ' ');
    line += title + "  " + t.priority;
    if (!t.tags.empty()) {
        line += "  #";
        for (size_t i = 0; i < t.tags.size(); ++i) line += (i ? " #" : "") + t.tags[i];
    }
    if (t.due_at.has_value()) line += "  due=" + std::to_string(*t.due_at);
    return line;
}

std::string render(const taskstore::ApiResult& result) {
    switch (result.index()) {
        case 0:
            return "accepted";
        case 1:
            return render_task(std::get<1>(result));
        case 2: {
            const auto& page = std::get<2>(result);
            std::string out;
            for (const auto& t : page.items) out += render_task(t) + "\n";
            return out + "(" + std::to_string(page.items.size()) + " shown, " +
                   std::to_string(page.total) + " total)";
        }
        default: {
            const auto& e = std::get<3>(result);
            return "error: " + e.code + ": " + e.message;
        }
    }
}

/// Opens a connection, writes one request frame and reads one response frame.
int dial(const std::string& addr) {
    const size_t colon = addr.rfind(':');
    if (colon == std::string::npos) throw std::runtime_error("addr must be host:port");
    const std::string host = addr.substr(0, colon);
    const std::string port = addr.substr(colon + 1);

    addrinfo hints{};
    hints.ai_family   = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;

    addrinfo* res = nullptr;
    if (::getaddrinfo(host.c_str(), port.c_str(), &hints, &res) != 0)
        throw std::runtime_error("cannot resolve " + addr);

    for (addrinfo* a = res; a != nullptr; a = a->ai_next) {
        const int fd = ::socket(a->ai_family, a->ai_socktype, a->ai_protocol);
        if (fd < 0) continue;
        if (::connect(fd, a->ai_addr, a->ai_addrlen) == 0) {
            ::freeaddrinfo(res);
            return fd;
        }
        ::close(fd);
    }
    ::freeaddrinfo(res);
    throw std::runtime_error("cannot connect to " + addr);
}

}  // namespace

int main(int argc, char** argv) {
    std::vector<std::string> args(argv + 1, argv + argc);

    std::string addr = "127.0.0.1:9977";
    if (args.size() >= 2 && args[0] == "--addr") {
        addr = args[1];
        args.erase(args.begin(), args.begin() + 2);
    }

    taskstore::Request req{};
    req.request_id = 1;
    try {
        req.op = parse_op(args);
    } catch (const std::exception& e) {
        std::cerr << e.what() << "\n\n" << USAGE << "\n";
        return 2;
    }

    int fd = -1;
    try {
        fd = dial(addr);
        taskstore::write_frame(fd, taskstore::request_marshal(req));
        const taskstore::Response resp = taskstore::response_unmarshal(taskstore::read_frame(fd));
        ::close(fd);

        std::cout << render(resp.result) << "\n";
        return resp.result.index() == 3 ? 1 : 0;
    } catch (const std::exception& e) {
        if (fd >= 0) ::close(fd);
        std::cerr << addr << ": " << e.what() << "\n";
        return 1;
    }
}
