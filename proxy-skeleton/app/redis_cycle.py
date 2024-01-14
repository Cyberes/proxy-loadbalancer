import redis

redis_cycler_db = redis.Redis(host='localhost', port=6379, db=9)


def redis_cycle(list_name):
    """
    Emulates itertools.cycle() but returns the complete shuffled list.
    :param list_name:
    :return:
    """
    pipeline = redis_cycler_db.pipeline()
    pipeline.lpop(list_name)
    to_move = pipeline.execute()[0]
    if not to_move:
        return []
    pipeline.rpush(list_name, to_move)
    pipeline.lrange(list_name, 0, -1)
    results = pipeline.execute()
    new_list = results[-1]
    return [x.decode('utf-8') for x in new_list]


def add_backend_cycler(list_name: str, new_elements: list):
    existing_elements = [i.decode('utf-8') for i in redis_cycler_db.lrange(list_name, 0, -1)]
    existing_set = set(existing_elements)

    with redis_cycler_db.pipeline() as pipe:
        # Add elements
        for element in new_elements:
            if element not in existing_set:
                pipe.rpush(list_name, element)

        # Remove elements
        for element in existing_set:
            if element not in new_elements:
                pipe.lrem(list_name, 0, element)

        pipe.execute()
