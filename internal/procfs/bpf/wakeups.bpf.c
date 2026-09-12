// wakeups.bpf.c — counts scheduler wakeups attributed to the WAKING task's tgid
// (RFC §13.3 permits waker- or awakened-attribution as long as it is documented
// and unambiguous; we choose the waker because the sched_wakeup tracepoint fires
// in the waker's context, giving its tgid directly via bpf_get_current_pid_tgid,
// with no CO-RE/BTF dependency).
//
// Self-contained: declares the few helpers by their stable IDs and uses a legacy
// map definition, so it compiles with only `clang -target bpf` (no kernel or
// libbpf headers). Compiled once and embedded in the Go binary; loaded at
// runtime by cilium/ebpf.

typedef unsigned int __u32;
typedef unsigned long long __u64;

#define SEC(name) __attribute__((section(name), used))
#define BPF_MAP_TYPE_HASH 1
#define BPF_ANY 0

struct bpf_map_def {
	__u32 type;
	__u32 key_size;
	__u32 value_size;
	__u32 max_entries;
	__u32 map_flags;
};

static void *(*bpf_map_lookup_elem)(void *map, const void *key) = (void *)1;
static long (*bpf_map_update_elem)(void *map, const void *key, const void *value, __u64 flags) = (void *)2;
static __u64 (*bpf_get_current_pid_tgid)(void) = (void *)14;

struct bpf_map_def SEC("maps") wakeups = {
	.type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = 65536,
};

SEC("tracepoint/sched/sched_wakeup")
int on_sched_wakeup(void *ctx) {
	__u32 tgid = bpf_get_current_pid_tgid() >> 32;
	__u64 *val = bpf_map_lookup_elem(&wakeups, &tgid);
	if (val) {
		__sync_fetch_and_add(val, 1);
	} else {
		__u64 one = 1;
		bpf_map_update_elem(&wakeups, &tgid, &one, BPF_ANY);
	}
	return 0;
}

char LICENSE[] SEC("license") = "GPL";
