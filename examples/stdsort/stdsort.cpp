#include <iostream>
#include <algorithm>
#include <vector>

#include <stdlib.h>

int main(int argc, char* argv[]) {
    
	if (argc < 2) {
		std::cout << "Usage: stdsort N" << std::endl;
		return -1;
	}
	
	size_t N = atoi(argv[1]);
    
    int *arr = new int[N];
    
    for (int i = 0; i < N; i++)
        arr[i] = random();
    
    std::sort(arr, arr+N);
    
    return 0;
}
