#include <iostream>
#include <algorithm>
#include <vector>

#include <stdlib.h>

int main(int argc, char* argv[]) {
    
	size_t N = argc < 2 ? 1000 : atoi(argv[1]);
	
    
    int *arr = new int[N];
    
    for (int i = 0; i < N; i++)
        arr[i] = random();
    
    for (int i = 0; i < N; i++)
    for (int j = i; j < N; j++) {
        if (arr[i] > arr[j]) {
            std::swap(arr[i], arr[j]);
        }
    }
    
    std::cout << "Bubble sorted " << N << " elements" << std::endl;
    
    return 0;
}
