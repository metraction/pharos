def fibonacci(data):
    # Extract n from input data or use attribute access
    n = data.n
    
    # Calculate Fibonacci sequence
    seq = list(range(n))
    for i in seq[2:]:
        seq[i] = seq[i-2] + seq[i-1]
    
    # Return the result as a map of maps
    return {"fibonacci_sequence": seq, "count": n}