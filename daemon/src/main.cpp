#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <ctime>
#include <fstream>
#include <iostream>
#include <numeric>
#include <string>
#include <thread>
#include <vector>

#include "metrics.grpc.pb.h"
#include <grpcpp/grpcpp.h>
#include <utils/utils.hpp>

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::ServerWriter;
using grpc::Status;
using metric_daemon::MetricService;
using metric_daemon::NodeSnapshot;
using metric_daemon::StreamRequest;

class CpuTracker {
private:
  size_t prev_idle = 0;
  size_t prev_total = 0;

  std::vector<size_t> get_cpu_times() {
    std::ifstream file("/proc/stat");
    std::string line;
    std::vector<size_t> times;

    if (std::getline(file, line)) {
      if (line.compare(0, 3, "cpu") == 0) {
        size_t t[10] = {0};
        int fields = sscanf(
            line.c_str(), "cpu %zu %zu %zu %zu %zu %zu %zu %zu %zu %zu", &t[0],
            &t[1], &t[2], &t[3], &t[4], &t[5], &t[6], &t[7], &t[8], &t[9]);

        for (int i = 0; i < fields; ++i) {
          times.push_back(t[i]);
        }
      }
    }
    return times;
  }

public:
  CpuTracker() {
    std::vector<size_t> times = get_cpu_times();
    if (times.size() >= 4) {
      prev_idle = times[3];
      if (times.size() >= 5) prev_idle += times[4];
      prev_total = std::accumulate(times.begin(), times.end(), 0ULL);
    }
  }

  double get_usage_percent() {
    std::vector<size_t> times = get_cpu_times();
    if (times.size() < 4) return 0.0;

    size_t idle = times[3];
    if (times.size() >= 5) {
      idle += times[4];
    }

    size_t total = std::accumulate(times.begin(), times.end(), 0ULL);
    size_t idle_delta = idle - prev_idle;
    size_t total_delta = total - prev_total;

    prev_idle = idle;
    prev_total = total;

    if (total_delta == 0) {
      return 0.0f;
    }
    double active_delta = static_cast<double>(total_delta - idle_delta);
    return utils::convert_to_percent(active_delta, total_delta);
  }
};

class MetricServiceImpl final : public MetricService::Service {
  Status StreamMetrics(ServerContext *context, const StreamRequest *request,
                       ServerWriter<NodeSnapshot> *writer) override {
    CpuTracker cpu;

    uint64_t sleep_ms =
        request->sample_rate_ms() > 0 ? request->sample_rate_ms() : 1000;
    std::cout << "broker connected on streaming daemon telementry node:  "
              << request->node_id() << " at " << sleep_ms << "ms intervals.\n";

    while (!context->IsCancelled()) {
      std::this_thread::sleep_for(std::chrono::milliseconds(sleep_ms));

      NodeSnapshot snapshot;
      snapshot.set_nodeid(request->node_id());
      auto now = std::chrono::system_clock::now();
      snapshot.set_timestamp_ns(
          std::chrono::duration_cast<std::chrono::nanoseconds>(
              now.time_since_epoch())
              .count());

      auto *cpu_metrics = snapshot.mutable_cpu();
      cpu_metrics->set_usage(cpu.get_usage_percent());

      writer->Write(snapshot);
    }

    std::cout << "Broker disconnected.\n";
    return Status::OK;
  }
};

int main() {
  std::string server_address("0.0.0.0:50051");
  MetricServiceImpl service;

  ServerBuilder builder;
  builder.AddListeningPort(server_address, grpc::InsecureServerCredentials());
  builder.RegisterService(&service);

  std::unique_ptr<Server> server(builder.BuildAndStart());
  std::cout << "daemon listening on " << server_address << std::endl;

  server->Wait();
  return 0;
}
