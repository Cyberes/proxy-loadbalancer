from flask import Flask
from flask import request

app = Flask(__name__)


@app.route('/', defaults={'path': ''})
@app.route('/<path:path>')
def get_my_ip(path):
    return request.remote_addr, 200


if __name__ == '__main__':
    app.run(host="0.0.0.0", port=7860)
