import time


def fib(n):
    a, b = 0, 1
    for _ in range(n):
        a, b = b, a + b
    return a


def main():
    name = "DelveUI"
    count = 0
    for i in range(1, 6):
        count += i
        msg = f"Step {i}: count={count}, name={name}"
        print(msg)
        time.sleep(0.1)

    result = fib(10)
    print(f"fib(10) = {result}")
    print(f"Done. Total: {count}")


if __name__ == "__main__":
    main()
