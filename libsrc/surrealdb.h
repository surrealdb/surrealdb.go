#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

#define sr_SR_NONE 0

#define sr_SR_CLOSED -1

#define sr_SR_ERROR -2

#define sr_SR_FATAL -3

typedef enum sr_action {
  SR_ACTION_CREATE,
  SR_ACTION_UPDATE,
  SR_ACTION_DELETE,
} sr_action;

typedef struct sr_opaque_object_internal_t sr_opaque_object_internal_t;

typedef struct sr_RpcStream sr_RpcStream;

/**
 * may be sent across threads, but must not be aliased
 */
typedef struct sr_stream_t sr_stream_t;

/**
 * The object representing a Surreal connection
 *
 * It is safe to be referenced from multiple threads
 * If any operation, on any thread returns SR_FATAL then the connection is poisoned and must not be used again.
 * (use will cause the program to abort)
 *
 * should be freed with sr_surreal_disconnect
 */
typedef struct sr_surreal_t sr_surreal_t;

/**
 * The object representing a Surreal connection
 *
 * It is safe to be referenced from multiple threads
 * If any operation, on any thread returns SR_FATAL then the connection is poisoned and must not be used again.
 * (use will cause the program to abort)
 *
 * should be freed with sr_surreal_disconnect
 */
typedef struct sr_surreal_rpc_t sr_surreal_rpc_t;

typedef char *sr_string_t;

typedef struct sr_object_t {
  struct sr_opaque_object_internal_t *_0;
} sr_object_t;

typedef enum sr_number_t_Tag {
  SR_NUMBER_INT,
  SR_NUMBER_FLOAT,
} sr_number_t_Tag;

typedef struct sr_number_t {
  sr_number_t_Tag tag;
  union {
    struct {
      int64_t sr_number_int;
    };
    struct {
      double sr_number_float;
    };
  };
} sr_number_t;

typedef struct sr_duration_t {
  uint64_t secs;
  uint32_t nanos;
} sr_duration_t;

typedef struct sr_uuid_t {
  uint8_t _0[16];
} sr_uuid_t;

typedef struct sr_bytes_t {
  uint8_t *arr;
  int len;
} sr_bytes_t;

typedef enum sr_id_t_Tag {
  SR_ID_NUMBER,
  SR_ID_STRING,
  SR_ID_ARRAY,
  SR_ID_OBJECT,
} sr_id_t_Tag;

typedef struct sr_id_t {
  sr_id_t_Tag tag;
  union {
    struct {
      int64_t sr_id_number;
    };
    struct {
      sr_string_t sr_id_string;
    };
    struct {
      struct sr_array_t *sr_id_array;
    };
    struct {
      struct sr_object_t sr_id_object;
    };
  };
} sr_id_t;

typedef struct sr_thing_t {
  sr_string_t table;
  struct sr_id_t id;
} sr_thing_t;

typedef enum sr_value_t_Tag {
  SR_VALUE_NONE,
  SR_VALUE_NULL,
  SR_VALUE_BOOL,
  SR_VALUE_NUMBER,
  SR_VALUE_STRAND,
  SR_VALUE_DURATION,
  SR_VALUE_DATETIME,
  SR_VALUE_UUID,
  SR_VALUE_ARRAY,
  SR_VALUE_OBJECT,
  SR_VALUE_BYTES,
  SR_VALUE_THING,
} sr_value_t_Tag;

typedef struct sr_value_t {
  sr_value_t_Tag tag;
  union {
    struct {
      bool sr_value_bool;
    };
    struct {
      struct sr_number_t sr_value_number;
    };
    struct {
      sr_string_t sr_value_strand;
    };
    struct {
      struct sr_duration_t sr_value_duration;
    };
    struct {
      sr_string_t sr_value_datetime;
    };
    struct {
      struct sr_uuid_t sr_value_uuid;
    };
    struct {
      struct sr_array_t *sr_value_array;
    };
    struct {
      struct sr_object_t sr_value_object;
    };
    struct {
      struct sr_bytes_t sr_value_bytes;
    };
    struct {
      struct sr_thing_t sr_value_thing;
    };
  };
} sr_value_t;

typedef struct sr_array_t {
  struct sr_value_t *arr;
  int len;
} sr_array_t;

/**
 * when code = 0 there is no error
 */
typedef struct sr_SurrealError {
  int code;
  sr_string_t msg;
} sr_SurrealError;

typedef struct sr_arr_res_t {
  struct sr_array_t ok;
  struct sr_SurrealError err;
} sr_arr_res_t;

typedef struct sr_option_t {
  bool strict;
  uint8_t query_timeout;
  uint8_t transaction_timeout;
} sr_option_t;

typedef struct sr_notification_t {
  struct sr_uuid_t query_id;
  enum sr_action action;
  struct sr_value_t data;
} sr_notification_t;

/**
 * connects to a local, remote, or embedded database
 *
 * if any function returns SR_FATAL, this must not be used (except to drop) (TODO: check this is safe) doing so will cause the program to abort
 *
 * # Examples
 *
 * ```c
 * sr_string_t err;
 * sr_surreal_t *db;
 *
 * // connect to in-memory instance
 * if (sr_connect(&err, &db, "mem://") < 0) {
 *     printf("error connecting to db: %s\n", err);
 *     return 1;
 * }
 *
 * // connect to surrealkv file
 * if (sr_connect(&err, &db, "surrealkv://test.skv") < 0) {
 *     printf("error connecting to db: %s\n", err);
 *     return 1;
 * }
 *
 * // connect to surrealdb server
 * if (sr_connect(&err, &db, "wss://localhost:8000") < 0) {
 *     printf("error connecting to db: %s\n", err);
 *     return 1;
 * }
 *
 * sr_surreal_disconnect(db);
 * ```
 */
int sr_connect(sr_string_t *err_ptr,
               struct sr_surreal_t **surreal_ptr,
               const char *endpoint);

/**
 * disconnect a database connection
 * note: the Surreal object must not be used after this function has been called
 *     any object allocations will still be valid, and should be freed, using the appropriate function
 * TODO: check if Stream can be freed after disconnection because of rt
 *
 * # Examples
 *
 * ```c
 * sr_surreal_t *db;
 * // connect
 * disconnect(db);
 * ```
 */
void sr_surreal_disconnect(struct sr_surreal_t *db);

/**
 * create a record
 *
 */
int sr_create(const struct sr_surreal_t *db,
              sr_string_t *err_ptr,
              struct sr_object_t **res_ptr,
              const char *resource,
              const struct sr_object_t *content);

/**
 * make a live selection
 * if successful sets *stream_ptr to be an exclusive reference to an opaque Stream object
 * which can be moved accross threads but not aliased
 *
 * # Examples
 *
 * sr_stream_t *stream;
 * if (sr_select_live(db, &err, &stream, "foo") < 0)
 * {
 *     printf("%s", err);
 *     return 1;
 * }
 *
 * sr_notification_t not ;
 * if (sr_stream_next(stream, &not ) > 0)
 * {
 *     sr_print_notification(&not );
 * }
 * sr_stream_kill(stream);
 */
int sr_select_live(const struct sr_surreal_t *db,
                   sr_string_t *err_ptr,
                   struct sr_stream_t **stream_ptr,
                   const char *resource);

int sr_query(const struct sr_surreal_t *db,
             sr_string_t *err_ptr,
             struct sr_arr_res_t **res_ptr,
             const char *query,
             const struct sr_object_t *vars);

/**
 * select a resource
 *
 * can be used to select everything from a table or a single record
 * writes values to *res_ptr, and returns number of values
 * result values are allocated by Surreal and must be freed with sr_free_arr
 *
 * # Examples
 *
 * ```c
 * sr_surreal_t *db;
 * sr_string_t err;
 * sr_value_t *foos;
 * int len = sr_select(db, &err, &foos, "foo");
 * if (len < 0) {
 *     printf("%s", err);
 *     return 1;
 * }
 * ```
 * for (int i = 0; i < len; i++)
 * {
 *     sr_value_print(&foos[i]);
 * }
 * sr_free_arr(foos, len);
 */
int sr_select(const struct sr_surreal_t *db,
              sr_string_t *err_ptr,
              struct sr_value_t **res_ptr,
              const char *resource);

/**
 * select database
 * NOTE: namespace must be selected first with sr_use_ns
 *
 * # Examples
 * ```c
 * sr_surreal_t *db;
 * sr_string_t err;
 * if (sr_use_db(db, &err, "test") < 0)
 * {
 *     printf("%s", err);
 *     return 1;
 * }
 * ```
 */
int sr_use_db(const struct sr_surreal_t *db, sr_string_t *err_ptr, const char *query);

/**
 * select namespace
 * NOTE: database must be selected before use with sr_use_db
 *
 * # Examples
 * ```c
 * sr_surreal_t *db;
 * sr_string_t err;
 * if (sr_use_ns(db, &err, "test") < 0)
 * {
 *     printf("%s", err);
 *     return 1;
 * }
 * ```
 */
int sr_use_ns(const struct sr_surreal_t *db, sr_string_t *err_ptr, const char *query);

/**
 * returns the db version
 * NOTE: version is allocated in Surreal and must be freed with sr_free_string
 * # Examples
 * ```c
 * sr_surreal_t *db;
 * sr_string_t err;
 * sr_string_t ver;
 *
 * if (sr_version(db, &err, &ver) < 0)
 * {
 *     printf("%s", err);
 *     return 1;
 * }
 * printf("%s", ver);
 * sr_free_string(ver);
 * ```
 */
int sr_version(const struct sr_surreal_t *db, sr_string_t *err_ptr, sr_string_t *res_ptr);

int sr_surreal_rpc_new(sr_string_t *err_ptr,
                       struct sr_surreal_rpc_t **surreal_ptr,
                       const char *endpoint,
                       struct sr_option_t options);

/**
 * execute rpc
 *
 * free result with sr_free_byte_arr
 */
int sr_surreal_rpc_execute(const struct sr_surreal_rpc_t *self,
                           sr_string_t *err_ptr,
                           uint8_t **res_ptr,
                           const uint8_t *ptr,
                           int len);

int sr_surreal_rpc_notifications(const struct sr_surreal_rpc_t *self,
                                 sr_string_t *err_ptr,
                                 struct sr_RpcStream **stream_ptr);

void sr_surreal_rpc_free(struct sr_surreal_rpc_t *ctx);

void sr_free_arr(struct sr_value_t *ptr, int len);

void sr_free_bytes(struct sr_bytes_t bytes);

void sr_free_byte_arr(uint8_t *ptr, int len);

void sr_print_notification(const struct sr_notification_t *notification);

const struct sr_value_t *sr_object_get(const struct sr_object_t *obj, const char *key);

struct sr_object_t sr_object_new(void);

void sr_object_insert(struct sr_object_t *obj, const char *key, const struct sr_value_t *value);

void sr_object_insert_str(struct sr_object_t *obj, const char *key, const char *value);

void sr_object_insert_int(struct sr_object_t *obj, const char *key, int value);

void sr_object_insert_float(struct sr_object_t *obj, const char *key, float value);

void sr_object_insert_double(struct sr_object_t *obj, const char *key, double value);

void sr_free_object(struct sr_object_t obj);

void sr_free_arr_res(struct sr_arr_res_t res);

void sr_free_arr_res_arr(struct sr_arr_res_t *ptr, int len);

/**
 * blocks until next item is recieved on stream
 * will return 1 and write notification to notification_ptr is recieved
 * will return SR_NONE if the stream is closed
 */
int sr_stream_next(struct sr_stream_t *self, struct sr_notification_t *notification_ptr);

void sr_stream_kill(struct sr_stream_t *stream);

void sr_free_string(sr_string_t string);

void sr_value_print(const struct sr_value_t *val);

bool sr_value_eq(const struct sr_value_t *lhs, const struct sr_value_t *rhs);
