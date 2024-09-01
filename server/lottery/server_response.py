OK = 1
ERROR = 0

class ServerResponse:
    @classmethod
    def ok_bytes(cls) -> bytes:
        return OK.to_bytes(1, byteorder='big')
    
    @classmethod
    def error_bytes(cls) -> bytes:
        return ERROR.to_bytes(1, byteorder='big')
    